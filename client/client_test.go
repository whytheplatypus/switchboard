package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/whytheplatypus/switchboard/operator"
)

// blackhole accepts the connection and then says nothing, the way a suspended
// machine or a firewall that drops rather than refuses does.
func blackhole(t *testing.T) string {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	return srv.URL + "/register"
}

// This is what stalled hookups: http.Post uses a client with no timeout, so
// one operator that never answers held the heartbeat forever and the route it
// was refreshing expired.
func TestPostGivesUp(t *testing.T) {
	if registrar.Timeout == 0 {
		t.Fatal("the registrar has no timeout, one silent operator will stall the heartbeat")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- post(ctx, blackhole(t), operator.Registration{Pattern: "http://x.local", Addr: "10.0.0.4:8000"})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a silent operator should be an error, not a success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("post did not give up on a silent operator")
	}
}

// Silent operators must not stop the healthy one from being refreshed. Taken
// in turn they would spend the whole budget between them and the good operator
// would never be called, whatever order they happened to sort in.
func TestSilentOperatorsDoNotStarveTheGoodOne(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	}))
	defer good.Close()

	known := &operators{}
	for i := 0; i < 4; i++ {
		known.found(blackhole(t))
	}
	known.found(good.URL + "/register")

	// less than the four silent operators would need one after another
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	if !register(ctx, known, operator.Registration{Pattern: "http://x.local", Addr: "10.0.0.4:8000"}) {
		t.Fatal("the reachable operator was starved by the silent ones")
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("the pass took %s, the operators were not called at once", took)
	}
}

// Discovery finds operators; it must not be what keeps them. A quiet moment on
// the multicast group would otherwise expire a live route.
func TestKnownOperatorsSurviveSilentDiscovery(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	}))
	defer good.Close()

	known := &operators{}
	known.found(good.URL + "/register")

	// no operator will answer the mdns query in a test, and it should not matter
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if !register(ctx, known, operator.Registration{Pattern: "http://x.local", Addr: "10.0.0.4:8000"}) {
		t.Fatal("a known operator was dropped because discovery found nothing")
	}
}

func TestOperatorsRetireAfterRepeatedFailure(t *testing.T) {
	known := &operators{}
	known.found("http://gone.local/register")

	for i := 0; i < forget-1; i++ {
		known.failed("http://gone.local/register")
		if len(known.all()) != 1 {
			t.Fatalf("retired an operator after %d failures, want %d", i+1, forget)
		}
	}
	known.failed("http://gone.local/register")
	if len(known.all()) != 0 {
		t.Fatal("an operator that never answers was never retired")
	}

	// discovery can always bring it back
	known.found("http://gone.local/register")
	if len(known.all()) != 1 {
		t.Fatal("a rediscovered operator was not taken back")
	}
}

// A success has to clear the count, or an operator that fails now and then
// eventually gets retired for no good reason.
func TestSuccessClearsFailures(t *testing.T) {
	known := &operators{}
	api := "http://flaky.local/register"
	known.found(api)

	for i := 0; i < forget*3; i++ {
		known.failed(api)
		known.worked(api)
	}
	if len(known.all()) != 1 {
		t.Fatal("a flaky but reachable operator was retired")
	}
}
