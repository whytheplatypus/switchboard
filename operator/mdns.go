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
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
)

type router interface {
	register(pattern string, target string)
}

func Listen(ctx context.Context, r router) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	wg := sync.WaitGroup{}
	entries := make(chan *mdns.ServiceEntry, 5)
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case entry := <-entries:
				slog.Info("registration", "pattern",
					entry.InfoFields[0], "ip",
					entry.AddrV4, "port",
					entry.Port,
				)
				if err := Connect(entry, r); err != nil {
					slog.Error("failed to connect", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	// Make a channel for results and start listening
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
	params.Timeout = time.Second * 5

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := mdns.QueryContext(ctx, params); err != nil {
					slog.Error("mdns query failed", "error", err)
				}
			}
		}
	}()
	wg.Wait()
}

func Connect(entry *mdns.ServiceEntry, r router) error {
	if !strings.Contains(entry.Name, config.ServiceName) {
		return ErrUnknownEntry
	}
	u := fmt.Sprintf("http://%s:%d", entry.AddrV4, entry.Port)

	if _, err := url.Parse(u); err != nil {
		return err
	}
	r.register(entry.InfoFields[0], u)
	return nil
}
