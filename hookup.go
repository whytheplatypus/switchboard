package main

import (
	"context"
	"flag"
	"net"

	"github.com/whytheplatypus/switchboard/client"
)

func hookup(args []string, ctx context.Context) {
	flags := flag.NewFlagSet("hookup", flag.ExitOnError)
	pattern := flags.String("pattern", "", "the url pattern that should forward to this service")
	port := flags.Int("port", 80, "the port the service runs on")
	flags.Parse(args)

	ips := flags.Args()
	ipss := []net.IP{}
	for _, ip := range ips {
		ipss = append(ipss, net.ParseIP(ip))
	}

	server := client.Hookup(*pattern, *port, ipss...)
	defer server.Shutdown()
	<-ctx.Done()
}
