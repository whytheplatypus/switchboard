package operator

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
)

var (
	ErrUnknownEntry   = errors.New("mdns: unkown entry type recieved")
	ErrDuplicateEntry = errors.New("mdns: duplicate entry recieved")
)

var defaultRouter = &Router{}

type phonebookIndex []string

func (i phonebookIndex) Len() int           { return len(i) }
func (i phonebookIndex) Swap(j, k int)      { i[j], i[k] = i[k], i[j] }
func (i phonebookIndex) Less(j, k int) bool { return len(i[j]) > len(i[k]) }

type Router struct {
	phonebook map[string]*url.URL
	index     phonebookIndex
	mu        sync.Mutex
	matcher   func(pattern *url.URL, requested *url.URL) bool
}

func (r *Router) register(pattern string, target *url.URL) {
	r.mu.Lock()
	if r.phonebook == nil {
		r.phonebook = map[string]*url.URL{}
	}
	defer r.mu.Unlock()
	r.phonebook[pattern] = target
	r.updateIndex()
}

func (r *Router) updateIndex() {
	r.index = phonebookIndex(make([]string, len(r.phonebook)))
	i := 0
	for k := range r.phonebook {
		r.index[i] = k
		i++
	}
	sort.Sort(r.index)
}

func (r *Router) direct(req *http.Request) {
	target, pattern := r.lookup(req)
	if target == nil {
		panic("No Target URL found")
	}
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
	r.mu.Lock()
	defer r.mu.Unlock()
	req.URL.Host = req.Host
	for _, v := range r.index {
		u, err := url.Parse(v)
		if err != nil {
			panic(err)
		}
		if r.match(u, req.URL) {
			return r.phonebook[v], u
		}
	}
	return nil, nil
}

func (r *Router) match(pattern *url.URL, requested *url.URL) bool {
	if r.matcher != nil {
		return r.matcher(pattern, requested)
	}
	return defaultMatch(pattern, requested)
}

func (r *Router) Handler() *httputil.ReverseProxy {
	proxy := &httputil.ReverseProxy{
		Director: r.direct,
	}
	return proxy
}

func Handler() *httputil.ReverseProxy {
	return defaultRouter.Handler()
}

func defaultMatch(p *url.URL, r *url.URL) bool {
	return (p.Host == "" || p.Host == r.Host) && strings.HasPrefix(r.Path, p.Path) && strings.Contains(r.RawQuery, p.RawQuery)
}
