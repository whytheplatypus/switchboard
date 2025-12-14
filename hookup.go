package main

import (
	"context"
	"flag"

	"github.com/whytheplatypus/switchboard/client"
	"github.com/whytheplatypus/switchboard/config"
)

func hookup(args []string, ctx context.Context) {
	flags := flag.NewFlagSet("hookup", flag.ExitOnError)
	pattern := flags.String("pattern", "", "the url pattern that should forward to this service")
	port := flags.Int("port", 80, "the port the service runs on")
	flags.StringVar(&config.Iface, "iface", "", "interface to listen on")
	flags.Parse(args)

	server := client.Hookup(*pattern, *port)
	defer server.Shutdown()
	<-ctx.Done()
}
