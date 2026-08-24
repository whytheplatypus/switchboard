package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
	"github.com/whytheplatypus/switchboard/operator"
)

// forget is how many passes in a row an operator has to fail before a hookup
// stops trying it. Discovery can always bring it back.
const forget = 3

// registrar bounds every call to an operator. Without a timeout an operator
// that accepts the connection and then says nothing -- a suspended machine, a
// firewall that drops instead of refusing -- holds the heartbeat forever, and
// the route it was refreshing quietly expires.
var registrar = &http.Client{Timeout: config.Reach}

// A Service is what this hookup asks operators to route to it. Anything the
// registration grows belongs here rather than in another argument.
type Service struct {
	Pattern string
	IP      net.IP
	Port    int
	// Auth, when set, is basic auth operators enforce on this route.
	Auth *operator.Auth
}

func (s Service) registration() operator.Registration {
	return operator.Registration{
		Pattern: s.Pattern,
		Addr:    net.JoinHostPort(s.IP.String(), fmt.Sprint(s.Port)),
		Auth:    s.Auth,
	}
}

// Hookup keeps this service registered with every operator on the network
// until ctx is cancelled.
func Hookup(ctx context.Context, svc Service) error {
	reg := svc.registration()

	summons := make(chan struct{}, 1)
	bell, err := newDoorbell(summons, svc.IP, svc.Port)
	if err != nil {
		return err
	}
	server, err := mdns.NewServer(&mdns.Config{
		Zone:   bell,
		Iface:  config.Interface(),
		Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		return err
	}
	defer server.Shutdown()

	known := &operators{}
	misses := 0
	for {
		started := time.Now()
		if register(ctx, known, reg) {
			misses = 0
		} else {
			misses++
			slog.Warn("Reached no operator", "passes", misses, "pattern", reg.Pattern)
		}

		// Anything that rang while the pass was running has been answered by
		// that very pass.
		select {
		case <-summons:
		default:
		}

		// A failed pass comes back quickly; a good one waits for the
		// heartbeat. Either way the wait is measured from where the pass
		// started, so a slow pass does not push the next one out.
		wait := config.Heartbeat
		if misses > 0 {
			wait = config.Retry
		}
		timer := time.NewTimer(time.Until(started.Add(wait)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		case <-summons:
			timer.Stop()
			// However eagerly the network rings, passes stay this far apart.
			if floor := time.Until(started.Add(config.Retry)); floor > 0 {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(floor):
				}
			}
		}
	}
}

// register tells every operator it knows of where to send this pattern, and
// reports whether any of them took it.
func register(ctx context.Context, known *operators, reg operator.Registration) bool {
	// A pass must never outlast a heartbeat or the next one never happens.
	ctx, cancel := context.WithTimeout(ctx, config.Heartbeat)
	defer cancel()

	discover(ctx, known)

	// Every operator is called at once. Taking them in turn means a slow one
	// spends the time the operators behind it needed, which is how a route
	// that had a perfectly healthy operator to refresh against still expires.
	var wg sync.WaitGroup
	var mu sync.Mutex
	worked, failed := []string{}, []string{}
	for _, api := range known.all() {
		wg.Add(1)
		go func(api string) {
			defer wg.Done()
			err := post(ctx, api, reg)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				slog.Error("Failed to register", "error", err, "operator", api)
				failed = append(failed, api)
				return
			}
			slog.Info("Registered", "pattern", reg.Pattern, "addr", reg.Addr, "guarded", reg.Auth != nil, "operator", api)
			worked = append(worked, api)
		}(api)
	}
	wg.Wait()

	// The bookkeeping happens back on one goroutine, so the set of known
	// operators needs no lock of its own.
	for _, api := range failed {
		known.failed(api)
	}
	for _, api := range worked {
		known.worked(api)
	}
	return len(worked) > 0
}

// discover adds every operator answering on the network to the known set.
func discover(ctx context.Context, known *operators) {
	found := make(chan *mdns.ServiceEntry, 5)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range found {
			api, err := endpoint(entry)
			if err != nil {
				slog.Error("Unusable operator", "error", err, "operator", entry.Name)
				continue
			}
			known.found(api)
		}
	}()

	params := mdns.DefaultParams(config.OperatorService)
	params.Entries = found
	params.Interface = config.Interface()
	params.Logger = log.New(io.Discard, "", 0)

	if err := mdns.QueryContext(ctx, params); err != nil {
		slog.Error("mdns query failed", "error", err)
	}
	// The query will not block on a full channel, it drops entries instead, so
	// the channel is only closed once the query is finished with it.
	close(found)
	<-done
}

func endpoint(entry *mdns.ServiceEntry) (string, error) {
	port := fmt.Sprint(entry.Port)
	switch {
	case entry.AddrV4 != nil:
		return fmt.Sprintf("http://%s/register", net.JoinHostPort(entry.AddrV4.String(), port)), nil
	case entry.AddrV6 != nil:
		return fmt.Sprintf("http://%s/register", net.JoinHostPort(entry.AddrV6.String(), port)), nil
	}
	return "", fmt.Errorf("operator %q announced no address", entry.Name)
}

func post(ctx context.Context, api string, reg operator.Registration) error {
	body, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := registrar.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s: %s", api, resp.Status)
	}
	return nil
}

// operators remembers where registrations have been accepted. Discovery is how
// operators are found, but it must not be how they are kept: a moment of
// multicast trouble would otherwise stop the heartbeat and let a live route
// expire. Repeated failure, not one silent lookup, is what retires an entry.
type operators struct {
	failures map[string]int
}

func (o *operators) found(api string) {
	if o.failures == nil {
		o.failures = map[string]int{}
	}
	if _, ok := o.failures[api]; !ok {
		slog.Info("Found operator", "operator", api)
	}
	o.failures[api] = 0
}

func (o *operators) worked(api string) {
	if o.failures != nil {
		o.failures[api] = 0
	}
}

func (o *operators) failed(api string) {
	if o.failures == nil {
		return
	}
	o.failures[api]++
	if o.failures[api] >= forget {
		slog.Warn("Giving up on operator", "operator", api, "passes", o.failures[api])
		delete(o.failures, api)
	}
}

// all is a snapshot, so an operator can be retired while it is being walked.
func (o *operators) all() []string {
	apis := make([]string, 0, len(o.failures))
	for api := range o.failures {
		apis = append(apis, api)
	}
	sort.Strings(apis)
	return apis
}
