package operator

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// guarded stands up a proxy in front of an echo service and registers one open
// route and one behind basic auth.
func guarded(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	r := &Router{}

	upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// report back whether the credential was passed upstream
		rw.Write([]byte("authorization:" + req.Header.Get("Authorization")))
	}))
	t.Cleanup(upstream.Close)

	proxy := r.Handler()
	h := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.NotFound(rw, req)
			}
		}()
		proxy.ServeHTTP(rw, req)
	})
	srv := httptest.NewServer(r.Guard(h))
	t.Cleanup(srv.Close)

	r.register(srv.URL+"/open", upstream.URL, nil, 0)
	r.register(srv.URL+"/locked", upstream.URL, &Auth{Username: "ada", Password: "s3cret"}, 0)
	return srv, srv.Client()
}

func TestGuard(t *testing.T) {
	srv, c := guarded(t)

	tests := []struct {
		name   string
		path   string
		user   string
		pass   string
		status int
	}{
		{"open route needs nothing", "/open", "", "", http.StatusOK},
		{"guarded route rejects anonymous", "/locked", "", "", http.StatusUnauthorized},
		{"guarded route rejects wrong password", "/locked", "ada", "wrong", http.StatusUnauthorized},
		{"guarded route rejects wrong user", "/locked", "eve", "s3cret", http.StatusUnauthorized},
		{"guarded route accepts the credential", "/locked", "ada", "s3cret", http.StatusOK},
		{"unregistered route is still a 404", "/nope", "", "", http.StatusNotFound},
		{"credentials do not open an unregistered route", "/nope", "ada", "s3cret", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tt.user != "" || tt.pass != "" {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			resp, err := c.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.status {
				t.Fatalf("got %s, want %d", resp.Status, tt.status)
			}
		})
	}
}

func TestGuardChallenges(t *testing.T) {
	srv, c := guarded(t)

	resp, err := c.Get(srv.URL + "/locked")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, "Basic realm=") {
		t.Fatalf("no basic auth challenge, got %q", got)
	}
}

// The credential is the switchboard's; the service behind it never issued it
// and should not be handed it.
func TestCredentialIsNotForwarded(t *testing.T) {
	srv, c := guarded(t)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/locked", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("ada", "s3cret")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "authorization:" {
		t.Fatalf("credential reached the service: %q", got)
	}
}

func TestAllowIgnoresNil(t *testing.T) {
	var open *Auth
	req := &http.Request{Header: http.Header{}, URL: &url.URL{}}
	if !open.allow(req) {
		t.Fatal("a route with no credential should allow anything")
	}
}
