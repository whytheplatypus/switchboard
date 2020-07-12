package operator

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
)

func Listen(ctx context.Context) <-chan *mdns.ServiceEntry {
	// Make a channel for results and start listening
	entries := make(chan *mdns.ServiceEntry, 5)

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
				mdns.Lookup(fmt.Sprintf("%s", config.ServiceName), entries)
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
