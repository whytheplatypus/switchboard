package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/whytheplatypus/switchboard/operator"
)

func route(args []string, ctx context.Context) {
	flags := flag.NewFlagSet("route", flag.ExitOnError)
	port := flags.Int("port", 80, "the port this should run on")
	cdir := flags.String("cert-directory", "/var/cache/switchboard/autocert", "the directory to store the acme cert")
	var domains StringArray
	flags.Var(&domains, "domain", "a domain to register a tls cert for")
	httpLog := flags.String("log-http", "", "The address to serve logs over, no logs are served if empty")
	flags.Parse(args)

	if *httpLog != "" {
		configureLog(*httpLog)
	}

	go func() {
		entries := operator.Listen(ctx)
		for entry := range entries {
			if err := operator.Connect(entry); err != nil {
				registrationLog.Println(err)
				continue
			}
			// register
			registrationLog.Printf(`{"send":"%s","to":"http://%s:%d"}`,
				entry.InfoFields[0],
				entry.AddrV4,
				entry.Port,
			)
		}
	}()

	router := operator.Handler()

	router.ModifyResponse = func(r *http.Response) error {
		info := struct {
			Host   string `json:"host"`
			Target string `json:"target"`
			Path   string `json:"path"`
			Query  string `json:"query"`
		}{
			r.Request.Host,
			r.Request.URL.Host,
			r.Request.URL.Path,
			r.Request.URL.RawQuery,
		}

		b, _ := json.Marshal(info)
		routingLog.Println(string(b))
		return nil
	}

	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.NotFound(rw, r)
			}
		}()
		router.ServeHTTP(rw, r)
	})

	srv := &server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: handlers.LoggingHandler(&lWriter{accessLog}, h),
		CertDir: *cdir,
		Domains: domains,
	}

	if err := srv.serve(ctx); err != nil {
		routingLog.Fatal(err)
	}
}
