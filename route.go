package main

import (
	"context"
	"flag"
	"fmt"
	"log"

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
			log.Printf("Got new entry: %+v\n", entry)
			if err := operator.Connect(entry); err != nil {
				log.Println(err)
			}
		}
	}()

	srv := &server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: handlers.LoggingHandler(log.Writer(), operator.Handler()),
		CertDir: *cdir,
		Domains: domains,
	}

	if err := srv.serve(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println("exiting")
}
