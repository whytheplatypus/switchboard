package operator

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/whytheplatypus/switchboard/config"
)

// A Registration is a hookup asking for a pattern to be forwarded to it. Addr
// is the explicit "ip:port" the hookup was told to advertise, not wherever the
// request happened to come from: the service being routed to is not always the
// process doing the registering.
type Registration struct {
	Pattern string `json:"pattern"`
	Addr    string `json:"addr"`
	// Auth, when set, is basic auth the switchboard enforces on this route.
	Auth *Auth `json:"auth,omitempty"`
}

// API serves the registration endpoint. It is the control plane and belongs on
// its own listener, never behind the proxy itself.
func (r *Router) API() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", r.handleRegister)
	return mux
}

func API() http.Handler {
	return DefaultRouter.API()
}

func (rt *Router) handleRegister(rw http.ResponseWriter, r *http.Request) {
	var reg Registration
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	if reg.Pattern == "" {
		http.Error(rw, "pattern is required", http.StatusBadRequest)
		return
	}
	if err := checkAddr(reg.Addr); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	if reg.Auth != nil && (reg.Auth.Username == "" || reg.Auth.Password == "") {
		http.Error(rw, "auth needs both a username and a password", http.StatusBadRequest)
		return
	}

	// Whatever else is logged here, never the password.
	slog.Info("registration", "pattern", reg.Pattern, "addr", reg.Addr, "guarded", reg.Auth != nil, "from", r.RemoteAddr)
	rt.register(reg.Pattern, fmt.Sprintf("http://%s", reg.Addr), reg.Auth, config.Lease)
	rw.WriteHeader(http.StatusNoContent)
}

// checkAddr insists on an explicit ip:port. A name here would be resolved on
// every request by the proxy, which is exactly what discovery is for.
func checkAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if net.ParseIP(host) == nil {
		return fmt.Errorf("addr %q: host must be an ip address", addr)
	}
	if port == "" {
		return fmt.Errorf("addr %q: port is required", addr)
	}
	return nil
}
