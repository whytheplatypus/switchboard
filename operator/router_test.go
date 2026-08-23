package operator

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func init() {
	log.SetFlags(log.Llongfile)
}

func TestHandler(t *testing.T) {
	router := DefaultRouter.Handler()
	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.NotFound(rw, r)
			}
		}()
		router.ServeHTTP(rw, r)
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	pathEchoSrv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(r.URL.String()))
	}))
	defer pathEchoSrv.Close()
	c := srv.Client()

	tests := []struct {
		pattern string
		target  string
		url     string
		result  string
	}{
		{
			"",
			"",
			fmt.Sprintf("%s/%s", srv.URL, "not-found"),
			"404 page not found\n",
		},
		{
			srv.URL,
			pathEchoSrv.URL,
			fmt.Sprintf("%s/%s", srv.URL, "test"),
			"/test",
		},
		{
			srv.URL,
			pathEchoSrv.URL,
			fmt.Sprintf("%s/%s", srv.URL, "test/"),
			"/test/",
		},
		{
			fmt.Sprintf("%s/%s", srv.URL, "test"),
			pathEchoSrv.URL,
			fmt.Sprintf("%s/%s", srv.URL, "test"),
			"/",
		},
		{
			fmt.Sprintf("%s/%s", srv.URL, "other"),
			pathEchoSrv.URL,
			fmt.Sprintf("%s/%s", srv.URL, "other/test"),
			"/test",
		},
	}
	for i, tt := range tests {
		if tt.pattern != "" {
			DefaultRouter.register(tt.pattern, tt.target, nil, 0)
		}
		resp, err := c.Get(tt.url)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tt.result {
			t.Fatal("wrong route, got", string(b), "wanted", tt.result, "test", i)
		}
	}
	if len(DefaultRouter.phonebook) > 3 {
		t.Fatal("duplicate entries were registered", len(DefaultRouter.phonebook))
	}
}
