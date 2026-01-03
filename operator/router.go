package operator

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

var (
	ErrUnknownEntry   = errors.New("mdns: unkown entry type recieved")
	ErrDuplicateEntry = errors.New("mdns: duplicate entry recieved")
)

var DefaultRouter = &Router{}

type Router struct {
	phonebook map[string]*url.URL // host names only
	mu        sync.Mutex
}

func (r *Router) register(pattern string, target *url.URL) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phonebook == nil {
		r.phonebook = map[string]*url.URL{}
	}
	r.phonebook[pattern] = target
	slog.Info("Registered route", "pattern", pattern, "target", target)
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
	pattern, err := url.Parse(fmt.Sprintf("%s://%s/%s", req.URL.Scheme, req.Host, prefix))
	if err != nil {
		slog.Error("Failed to parse URL", "error", err)
		return nil, nil
	}
	if target, ok := r.phonebook[pattern.String()]; ok {
		return target, pattern
	}
	pattern, err = url.Parse(fmt.Sprintf("%s://%s", req.URL.Scheme, req.Host))
	if err != nil {
		slog.Error("Failed to parse URL", "error", err)
		return nil, nil
	}
	if target, ok := r.phonebook[pattern.String()]; ok {
		return target, pattern
	}

	return nil, nil
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
