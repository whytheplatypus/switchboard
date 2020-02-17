package operator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/artistedev/switchboard/config"
	"github.com/hashicorp/mdns"
)

var Phonebook = &defaultServeMux

var defaultServeMux http.ServeMux

var registry = map[string]*mdns.ServiceEntry{}

func Listen(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	// Make a channel for results and start listening
	entriesCh := make(chan *mdns.ServiceEntry, 5)

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
	}(entriesCh)
	for entry := range entriesCh {
		fmt.Printf("Got new entry: %+v\n", entry)
		Connect(entry)
	}
	fmt.Println("Done connecting")
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
	u, _ := url.Parse(fmt.Sprintf("http://%s:%d", entry.AddrV4, entry.Port))
	rp := httputil.NewSingleHostReverseProxy(u)
	Phonebook.Handle(entry.InfoFields[0], rp)
	registry[entry.InfoFields[0]] = entry
}
