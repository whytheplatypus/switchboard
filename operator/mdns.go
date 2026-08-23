package operator

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
)

// Announce publishes where this operator's registration API can be reached.
// Hookups look this up to find somewhere to register.
func Announce(ctx context.Context, port int, ips ...net.IP) error {
	instance, err := os.Hostname()
	if err != nil {
		return err
	}
	service, err := mdns.NewMDNSService(instance, config.OperatorService, "", "", port, ips, nil)
	if err != nil {
		return err
	}
	server, err := mdns.NewServer(&mdns.Config{
		Zone:   service,
		Iface:  config.Interface(),
		Logger: log.New(io.Discard, "", 0),
	})
	if err != nil {
		return err
	}
	slog.Info("Announcing registration api", "service", config.OperatorService, "port", port)
	go func() {
		<-ctx.Done()
		server.Shutdown()
	}()
	return nil
}

// Summon asks every hookup on the network to register itself. Nothing has to
// answer -- the question is the message -- so the replies are thrown away. It
// repeats so that a hookup which starts, reboots, or rejoins the network after
// this operator did still gets asked.
func Summon(ctx context.Context) {
	ticker := time.NewTicker(config.Summon)
	defer ticker.Stop()
	for {
		ask(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func ask(ctx context.Context) {
	// The query will not block on a full channel, it drops entries instead, so
	// the answers have to be drained even though they say nothing we need.
	answers := make(chan *mdns.ServiceEntry, 5)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range answers {
		}
	}()

	params := mdns.DefaultParams(config.HookupService)
	params.Entries = answers
	params.Interface = config.Interface()
	params.Logger = log.New(io.Discard, "", 0)

	slog.Info("Summoning hookups", "service", config.HookupService)
	if err := mdns.QueryContext(ctx, params); err != nil {
		slog.Error("mdns query failed", "error", err)
	}
	close(answers)
	<-done
}
