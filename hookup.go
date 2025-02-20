package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/whytheplatypus/switchboard/client"
)

func hookup(args []string, ctx context.Context) {
	flags := flag.NewFlagSet("hookup", flag.ExitOnError)
	pattern := flags.String("pattern", "", "the url pattern that should forward to this service")
	addr := flags.String("addr", ":80", "the address the service runs on")
	flags.Parse(args)

	host, p, err := net.SplitHostPort(*addr)

	ips := []net.IP{}
	ip := net.ParseIP(host)
	if ip != nil {
		ips = append(ips, ip)
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	server := client.Hookup(*pattern, port, ips...)
	defer server.Shutdown()
	<-ctx.Done()
}
