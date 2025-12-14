package main

import (
	"context"
	"net/http"
	"os"

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
		srv.Handler = m.HTTPHandler(nil)

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
