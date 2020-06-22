package operator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
)

var Phonebook = &defaultServeMux

var defaultServeMux http.ServeMux

var registry = map[string]*mdns.ServiceEntry{}

func Listen(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	// Make a channel for results and start listening
	entries := make(chan *mdns.ServiceEntry, 5)

	// Start the lookup
	go func(entries chan *mdns.ServiceEntry) {
		defer close(entries)
		defer fmt.Println("Done listening")
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mdns.Lookup(fmt.Sprintf("%s", config.ServiceName), entries)
			}
		}
	}(entries)
	for entry := range entries {
		fmt.Printf("Got new entry: %+v\n", entry)
		Connect(entry)
	}
}

func Connect(entry *mdns.ServiceEntry) {
	if !strings.Contains(entry.Name, config.ServiceName) {
		fmt.Println("unknown entry")
		return
	}
	if existing, ok := registry[entry.InfoFields[0]]; ok {
		if existing.AddrV4.Equal(entry.AddrV4) && existing.Port == entry.Port {
			return
		}
		*Phonebook = http.ServeMux{}
		delete(registry, entry.InfoFields[0])
		for _, ent := range registry {
			register(ent)
		}
	}
	register(entry)
}

func register(entry *mdns.ServiceEntry) {
	if _, ok := registry[entry.InfoFields[0]]; ok {
		return
	}

	u, _ := url.Parse(fmt.Sprintf("http://%s:%d", entry.AddrV4, entry.Port))
	var handler http.Handler
	handler = httputil.NewSingleHostReverseProxy(u)
	parts := strings.SplitN(entry.InfoFields[0], "/", 2)
	if len(parts) > 1 && parts[1] != "" {
		handler = http.StripPrefix(fmt.Sprintf("/%s", parts[1]), handler)
	}
	Phonebook.Handle(entry.InfoFields[0], handler)
	registry[entry.InfoFields[0]] = entry
}
