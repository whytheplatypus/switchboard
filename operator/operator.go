package operator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
)

var (
	ErrUnknownEntry   = errors.New("mdns: unkown entry type recieved")
	ErrDuplicateEntry = errors.New("mdns: duplicate entry recieved")
)

var defaultServeMux http.ServeMux

var Phonebook = &defaultServeMux

var registry = map[string]*mdns.ServiceEntry{}

func init() {
	Phonebook.HandleFunc("/", notFound)
}

func notFound(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func Listen(ctx context.Context, entries chan *mdns.ServiceEntry) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Start the lookup
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mdns.Lookup(fmt.Sprintf("%s", config.ServiceName), entries)
		}
	}
}

func Connect(entry *mdns.ServiceEntry) error {
	if !strings.Contains(entry.Name, config.ServiceName) {
		return ErrUnknownEntry
	}
	if existing, ok := registry[entry.InfoFields[0]]; ok {
		if existing.AddrV4.Equal(entry.AddrV4) && existing.Port == entry.Port {
			return ErrDuplicateEntry
		}
		*Phonebook = http.ServeMux{}
		delete(registry, entry.InfoFields[0])
		for _, ent := range registry {
			register(ent)
		}
	}
	register(entry)
	return nil
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
