package operator

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
)

func Listen(ctx context.Context) <-chan *mdns.ServiceEntry {
	// Make a channel for results and start listening
	entries := make(chan *mdns.ServiceEntry, 5)
	params := mdns.DefaultParams(config.ServiceName)
	if config.Iface != "" {
		if iface, err := net.InterfaceByName(config.Iface); err == nil {
			slog.Info("Using interface provided", "interface", iface.Name)
			params.Interface = iface
		} else {
			slog.Error("failed to get interface", "error", err)
		}
	}
	params.Logger = log.New(io.Discard, "", 0)
	params.Entries = entries

	// Start the lookup
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		defer close(entries)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := mdns.Query(params); err != nil {
					slog.Error("mdns query failed", "error", err)
				}
			}
		}
	}()

	return entries
}

func Connect(entry *mdns.ServiceEntry) error {
	if !strings.Contains(entry.Name, config.ServiceName) {
		return ErrUnknownEntry
	}
	return register(entry)
}

func register(entry *mdns.ServiceEntry) error {

	u, err := url.Parse(fmt.Sprintf("http://%s:%d", entry.AddrV4, entry.Port))
	if err != nil {
		return err
	}
	defaultRouter.register(entry.InfoFields[0], u)
	return nil
}
