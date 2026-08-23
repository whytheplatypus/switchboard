package operator

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
)

// Auth is the basic auth a hookup asks the switchboard to enforce on its
// behalf. It guards the route, not the service: the credential belongs to the
// switchboard and is never passed upstream.
type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// allow reports whether a request carries this credential. A nil Auth is an
// unguarded route, which allows everything.
func (a *Auth) allow(r *http.Request) bool {
	if a == nil {
		return true
	}
	user, pass, ok := r.BasicAuth()
	if !ok {
		return false
	}
	// Compare both halves before deciding, so a wrong username costs the same
	// as a wrong password.
	sameUser := subtle.ConstantTimeCompare([]byte(user), []byte(a.Username))
	samePass := subtle.ConstantTimeCompare([]byte(pass), []byte(a.Password))
	return sameUser == 1 && samePass == 1
}

// Guard turns away requests for a route whose hookup registered credentials.
// Routes registered without them are passed straight through, as is anything
// that matches no route at all -- that is the proxy's 404 to give.
func (r *Router) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		e, pattern := r.find(req)
		if e == nil || e.auth.allow(req) {
			next.ServeHTTP(rw, req)
			return
		}
		slog.Info("Rejecting unauthorized request", "pattern", pattern, "host", req.Host, "path", req.URL.Path)
		rw.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", pattern))
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
	})
}

func Guard(next http.Handler) http.Handler {
	return DefaultRouter.Guard(next)
}
