package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"slices"

	"golang.org/x/crypto/acme/autocert"
)

type server struct {
	Addr    string
	Handler http.Handler
	CertDir string
	Domains []string
}

func (s *server) serve(ctx context.Context) error {

	srv := &http.Server{
		Addr:    s.Addr,
		Handler: s.Handler,
	}

	if s.CertDir != "" && len(s.Domains) > 0 {
		m := &autocert.Manager{
			Prompt: autocert.AcceptTOS,
		}

		m.HostPolicy = autocert.HostWhitelist(s.Domains...)

		if err := os.MkdirAll(s.CertDir, os.ModePerm); err != nil {
			return err
		}
		m.Cache = autocert.DirCache(s.CertDir)
		srv.Handler = m.HTTPHandler(http.HandlerFunc(s.HTTPSChallengeFallbackHandler))

		crtSrv := &http.Server{
			Addr:      ":443",
			Handler:   s.Handler,
			TLSConfig: m.TLSConfig(),
		}
		//TODO return errors
		go crtSrv.ListenAndServeTLS("", "")
		defer crtSrv.Shutdown(context.Background())
	}

	//TODO return errors
	go srv.ListenAndServe()
	<-ctx.Done()
	//TODO return errors
	srv.Shutdown(context.Background())
	return nil
}

func (s *server) HTTPSChallengeFallbackHandler(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if slices.Contains(s.Domains, host) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Use HTTPS", http.StatusBadRequest)
			return
		}
		target := "https://" + stripPort(r.Host) + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	s.Handler.ServeHTTP(w, r)
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return net.JoinHostPort(host, "443")
}
