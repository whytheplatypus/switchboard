package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"strconv"

	"github.com/whytheplatypus/switchboard/client"
	"github.com/whytheplatypus/switchboard/config"
)

func hookup(args []string, ctx context.Context) {
	flags := flag.NewFlagSet("hookup", flag.ExitOnError)
	pattern := flags.String("pattern", "", "the url pattern that should forward to this service")
	addr := flags.String("addr", "127.0.0.1:80", "the address the service runs on")
	flags.StringVar(&config.Iface, "iface", "", "interface to listen on")
	flags.Parse(args)

	host, p, err := net.SplitHostPort(*addr)
	if err != nil {
		slog.Error("Failed to parse address", err)
		os.Exit(1)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		slog.Error("Failed to parse IP", err)
		os.Exit(1)
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		slog.Error("Failed to parse port", err)
		os.Exit(1)
	}

	server := client.Hookup(*pattern, port, ip)
	if server == nil {
		slog.Error("Failed to create server")
		os.Exit(1)
	}
	defer server.Shutdown()
	<-ctx.Done()
}
