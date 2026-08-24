package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/whytheplatypus/switchboard/config"
	"github.com/whytheplatypus/switchboard/operator"
)

func route(args []string, ctx context.Context) {
	flags := flag.NewFlagSet("route", flag.ContinueOnError)
	port := flags.Int("port", 80, "the port this should run on")
	cdir := flags.String("cert-directory", "/var/cache/switchboard/autocert", "the directory to store the acme cert")
	var domains StringArray
	flags.Var(&domains, "domain", "a domain to register a tls cert for")
	httpLog := flags.String("log-http", "", "The address to serve logs over, no logs are served if empty")
	apiPort := flags.Int("api-port", 4444, "the port to serve the registration api on")
	flags.StringVar(&config.Iface, "iface", "", "interface to listen on")
	if err := flags.Parse(args); err != nil && !strings.HasPrefix(err.Error(), "flag provided but not defined") {
		log.Fatal(err)
	}

	if *httpLog != "" {
		configureLog(*httpLog)
	}

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
		Handler: handlers.LoggingHandler(&lWriter{accessLog}, operator.Guard(h)),
		CertDir: *cdir,
		Domains: domains,
	}

	// The control plane: say where to register, ask everyone to do so, and
	// listen for them. None of it touches the proxy's own listener.
	if err := operator.Announce(ctx, *apiPort, config.Addresses()...); err != nil {
		slog.Error("Failed to announce registration api", "error", err)
		os.Exit(1)
	}
	go operator.Summon(ctx)
	go func() {
		api := &server{
			Addr:    fmt.Sprintf(":%d", *apiPort),
			Handler: operator.API(),
		}
		if err := api.serve(ctx); err != nil {
			slog.Error("registration api error", "error", err)
		}
	}()

	if err := srv.serve(ctx); err != nil {
		routingLog.Fatal(err)
	}
}
