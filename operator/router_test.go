package operator

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func init() {
	log.SetFlags(log.Llongfile)
}

func parseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestHandler(t *testing.T) {
	h := defaultRouter.Handler()
	srv := httptest.NewServer(h)
	defer srv.Close()
	pathEchoSrv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(r.URL.String()))
	}))
	defer pathEchoSrv.Close()
	c := srv.Client()

	tests := []struct {
		pattern string
		target  *url.URL
		url     string
		result  string
	}{
		{
			"",
			nil,
			fmt.Sprintf("%s/%s", srv.URL, "not-found"),
			"404 page not found\n",
		},
		{
			"/test",
			parseURL(pathEchoSrv.URL),
			fmt.Sprintf("%s/%s", srv.URL, "test"),
			"/",
		},
		{
			srv.URL,
			parseURL(pathEchoSrv.URL),
			fmt.Sprintf("%s/%s", srv.URL, "test"),
			"/test",
		},
		{
			fmt.Sprintf("%s/%s", srv.URL, "other/"),
			parseURL(pathEchoSrv.URL),
			fmt.Sprintf("%s/%s", srv.URL, "other/test"),
			"/test",
		},
		{
			srv.URL,
			parseURL(pathEchoSrv.URL),
			fmt.Sprintf("%s/%s", srv.URL, "test/"),
			"/test/",
		},
	}
	for _, tt := range tests {
		if tt.pattern != "" {
			defaultRouter.register(tt.pattern, tt.target)
		}
		resp, err := c.Get(tt.url)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tt.result {
			t.Fatal("wrong route, got", string(b))
		}
	}
	if len(defaultRouter.index) > 3 {
		t.Fatal("duplicate entries were registered", len(defaultRouter.index))
	}
}
