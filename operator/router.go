package operator

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

var DefaultRouter = &Router{}

// An extension is a target that stops answering once its lease runs out. A
// hookup keeps its extension alive by re-registering; anything that dies, is
// unplugged, or loses the network falls out of the phonebook on its own.
type extension struct {
	target  string
	expires time.Time
}

// A zero expiry never runs out, which is what a permanent registration wants.
func (e *extension) live() bool {
	return e.expires.IsZero() || time.Now().Before(e.expires)
}

type Router struct {
	phonebook map[string]*extension
	mu        sync.Mutex
}

func (r *Router) register(pattern string, target string, lease time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phonebook == nil {
		r.phonebook = map[string]*extension{}
	}

	// A heartbeat has to extend the lease even when nothing else changed, so
	// only the logging is skipped for a repeat, never the registration.
	if e, ok := r.phonebook[pattern]; !ok || e.target != target {
		slog.Info("Registered route", "pattern", pattern, "target", target, "lease", lease)
	}

	e := &extension{target: target}
	if lease > 0 {
		e.expires = time.Now().Add(lease)
	}
	r.phonebook[pattern] = e
	// Every heartbeat is a chance to sweep, so no timer is needed to keep the
	// phonebook from filling with the dead.
	r.forget()
}

// forget drops expired extensions. Callers must hold the lock.
func (r *Router) forget() {
	for pattern, e := range r.phonebook {
		if !e.live() {
			slog.Info("Forgetting expired route", "pattern", pattern, "target", e.target)
			delete(r.phonebook, pattern)
		}
	}
}

func (r *Router) direct(req *http.Request) {
	target, pattern := r.lookup(req)
	if target == nil {
		panic("No Target URL found")
	}
	slog.Info("Directing request", "pattern", pattern, "target", target, "request.host", req.Host, "request", req.URL.String())
	targetQuery := target.RawQuery
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.URL.Path = strings.TrimPrefix(req.URL.Path, pattern.Path)
	if !strings.HasPrefix(req.URL.Path, "/") {
		req.URL.Path = fmt.Sprintf("/%s", req.URL.Path)
	}
	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
	if _, ok := req.Header["User-Agent"]; !ok {
		// explicitly disable User-Agent so it's not set to default value
		req.Header.Set("User-Agent", "")
	}

}

func (r *Router) lookup(req *http.Request) (*url.URL, *url.URL) {
	slog.Info("Looking up route", "host", req.Host, "path", req.URL.Path)
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	prefix := strings.TrimPrefix(req.URL.Path, "/")
	prefix, _, _ = strings.Cut(prefix, "/")

	r.mu.Lock()
	defer r.mu.Unlock()
	// The longer pattern wins, so try host and first path segment first.
	if target, pattern := r.dial(fmt.Sprintf("%s://%s/%s", req.URL.Scheme, req.Host, prefix)); target != nil {
		return target, pattern
	}
	return r.dial(fmt.Sprintf("%s://%s", req.URL.Scheme, req.Host))
}

// dial resolves one pattern to its target, if it is registered and still
// holds a live lease. Callers must hold the lock.
func (r *Router) dial(pattern string) (*url.URL, *url.URL) {
	p, err := url.Parse(pattern)
	if err != nil {
		slog.Error("Failed to parse URL", "error", err)
		return nil, nil
	}
	e, ok := r.phonebook[p.String()]
	if !ok || !e.live() {
		return nil, nil
	}
	target, err := url.Parse(e.target)
	if err != nil {
		slog.Error("Failed to parse URL", "error", err)
		return nil, nil
	}
	return target, p
}

func (r *Router) Handler() *httputil.ReverseProxy {
	proxy := &httputil.ReverseProxy{
		Director: r.direct,
	}
	return proxy
}

func Handler() *httputil.ReverseProxy {
	return DefaultRouter.Handler()
}
