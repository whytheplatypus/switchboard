package operator

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRegister(t *testing.T) {
	r := &Router{}
	api := httptest.NewServer(r.API())
	defer api.Close()

	tests := []struct {
		body   string
		status int
	}{
		{`{"pattern":"http://example.local/thing","addr":"10.0.0.4:8000"}`, http.StatusNoContent},
		{`{"pattern":"http://example.local/thing"}`, http.StatusBadRequest},
		{`{"addr":"10.0.0.4:8000"}`, http.StatusBadRequest},
		// an explicit ip is required, a name is not good enough
		{`{"pattern":"http://example.local/thing","addr":"somebox.local:8000"}`, http.StatusBadRequest},
		{`{"pattern":"http://example.local/thing","addr":"10.0.0.4"}`, http.StatusBadRequest},
		{`not json`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		resp, err := http.Post(api.URL+"/register", "application/json", strings.NewReader(tt.body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tt.status {
			t.Fatalf("%s: got %s, want %d", tt.body, resp.Status, tt.status)
		}
	}

	e, ok := r.phonebook["http://example.local/thing"]
	if !ok {
		t.Fatal("registration was not recorded")
	}
	if want := "http://10.0.0.4:8000"; e.target != want {
		t.Fatalf("got %s, want %s", e.target, want)
	}
	if !e.live() {
		t.Fatal("a fresh registration should be live")
	}
}

func TestGetIsRejected(t *testing.T) {
	api := httptest.NewServer((&Router{}).API())
	defer api.Close()

	resp, err := http.Get(api.URL + "/register")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatal("got", resp.Status)
	}
}

func TestLeasesExpire(t *testing.T) {
	r := &Router{}
	r.register("http://expiring.local", "http://10.0.0.4:8000", nil, time.Millisecond)
	r.register("http://forever.local", "http://10.0.0.5:8001", nil, 0)

	if !r.phonebook["http://expiring.local"].live() {
		t.Fatal("a fresh lease should be live")
	}
	time.Sleep(2 * time.Millisecond)
	if r.phonebook["http://expiring.local"].live() {
		t.Fatal("an expired lease should not be live")
	}

	// Registering sweeps the dead, so no timer is needed.
	r.register("http://forever.local", "http://10.0.0.5:8001", nil, 0)
	if _, ok := r.phonebook["http://expiring.local"]; ok {
		t.Fatal("an expired extension should have been forgotten")
	}
}

// A heartbeat repeats the same pattern and target, and must still push the
// expiry out -- the noop shortcut that made sense without leases would have
// let a live service quietly expire.
func TestHeartbeatExtendsLease(t *testing.T) {
	r := &Router{}
	r.register("http://beating.local", "http://10.0.0.4:8000", nil, 20*time.Millisecond)
	first := r.phonebook["http://beating.local"].expires

	time.Sleep(5 * time.Millisecond)
	r.register("http://beating.local", "http://10.0.0.4:8000", nil, 20*time.Millisecond)

	if !r.phonebook["http://beating.local"].expires.After(first) {
		t.Fatal("a repeat registration did not extend the lease")
	}
}

func TestRegisterAuth(t *testing.T) {
	r := &Router{}
	api := httptest.NewServer(r.API())
	defer api.Close()

	tests := []struct {
		body   string
		status int
	}{
		{`{"pattern":"http://guarded.local","addr":"10.0.0.4:8000","auth":{"username":"ada","password":"s3cret"}}`, http.StatusNoContent},
		// half a credential would silently serve an open route
		{`{"pattern":"http://half.local","addr":"10.0.0.4:8000","auth":{"username":"ada"}}`, http.StatusBadRequest},
		{`{"pattern":"http://half.local","addr":"10.0.0.4:8000","auth":{"password":"s3cret"}}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		resp, err := http.Post(api.URL+"/register", "application/json", strings.NewReader(tt.body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tt.status {
			t.Fatalf("%s: got %s, want %d", tt.body, resp.Status, tt.status)
		}
	}

	auth := r.phonebook["http://guarded.local"].auth
	if auth == nil {
		t.Fatal("credential was not recorded")
	}
	if auth.Username != "ada" || auth.Password != "s3cret" {
		t.Fatalf("wrong credential recorded: %+v", auth)
	}
	if _, ok := r.phonebook["http://half.local"]; ok {
		t.Fatal("a half credential should not have registered a route")
	}
}
