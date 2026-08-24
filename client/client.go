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
	"os"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
	"github.com/whytheplatypus/switchboard/config"
	"github.com/whytheplatypus/switchboard/operator"
)

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
// until ctx is cancelled. It holds no list of operators: each pass rediscovers
// them, so an operator that restarts, moves, or appears for the first time is
// picked up by the very next pass.
func Hookup(ctx context.Context, svc Service) error {
	reg := svc.registration()

	summons := make(chan struct{}, 1)
	server, err := answer(summons, svc.IP, svc.Port)
	if err != nil {
		return err
	}
	defer server.Shutdown()

	ticker := time.NewTicker(config.Heartbeat)
	defer ticker.Stop()
	for {
		register(ctx, reg)
		select {
		case <-ctx.Done():
			return nil
		case <-summons:
			// An operator just came up; don't make it wait for the heartbeat.
		case <-ticker.C:
		}
	}
}

// answer runs the mDNS service an operator summons. Its records are beside the
// point -- the service exists so that the question reaches us.
func answer(summons chan<- struct{}, ip net.IP, port int) (*mdns.Server, error) {
	instance, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	service, err := mdns.NewMDNSService(instance, config.HookupService, "", "", port, []net.IP{ip}, nil)
	if err != nil {
		return nil, err
	}
	return mdns.NewServer(&mdns.Config{
		Zone:   &doorbell{service, summons},
		Iface:  config.Interface(),
		Logger: log.New(io.Discard, "", 0),
	})
}

// A doorbell is an mDNS zone that reports being asked about. The server hands
// it every question on the network, so it listens for its own name only.
type doorbell struct {
	*mdns.MDNSService
	summons chan<- struct{}
}

func (d *doorbell) Records(q dns.Question) []dns.RR {
	if strings.HasPrefix(q.Name, config.HookupService) {
		select {
		case d.summons <- struct{}{}:
		default: // one pending summons is as good as ten
		}
	}
	return d.MDNSService.Records(q)
}

// register tells every operator it can find where to send this pattern.
func register(ctx context.Context, reg operator.Registration) {
	operators := make(chan *mdns.ServiceEntry, 5)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range operators {
			api, err := endpoint(entry)
			if err != nil {
				slog.Error("unusable operator", "error", err, "operator", entry.Name)
				continue
			}
			if err := post(api, reg); err != nil {
				slog.Error("failed to register", "error", err, "operator", api)
				continue
			}
			slog.Info("registered", "pattern", reg.Pattern, "addr", reg.Addr, "guarded", reg.Auth != nil, "operator", api)
		}
	}()

	params := mdns.DefaultParams(config.OperatorService)
	params.Entries = operators
	params.Interface = config.Interface()
	params.Logger = log.New(io.Discard, "", 0)

	if err := mdns.QueryContext(ctx, params); err != nil {
		slog.Error("mdns query failed", "error", err)
	}
	close(operators)
	<-done
}

func endpoint(entry *mdns.ServiceEntry) (string, error) {
	switch {
	case entry.AddrV4 != nil:
		return fmt.Sprintf("http://%s/register", net.JoinHostPort(entry.AddrV4.String(), fmt.Sprint(entry.Port))), nil
	case entry.AddrV6 != nil:
		return fmt.Sprintf("http://%s/register", net.JoinHostPort(entry.AddrV6.String(), fmt.Sprint(entry.Port))), nil
	}
	return "", fmt.Errorf("operator %q announced no address", entry.Name)
}

func post(api string, reg operator.Registration) error {
	body, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	resp, err := http.Post(api, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s: %s", api, resp.Status)
	}
	return nil
}
