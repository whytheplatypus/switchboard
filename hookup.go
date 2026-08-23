package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"os"
	"strconv"

	"github.com/whytheplatypus/switchboard/client"
	"github.com/whytheplatypus/switchboard/config"
	"github.com/whytheplatypus/switchboard/operator"
)

func hookup(args []string, ctx context.Context) {
	flags := flag.NewFlagSet("hookup", flag.ExitOnError)
	pattern := flags.String("pattern", "", "the url pattern that should forward to this service")
	addr := flags.String("addr", "127.0.0.1:80", "the address the service runs on")
	user := flags.String("basic-auth-user", "", "username to require on this route, or $SWITCHBOARD_BASIC_AUTH_USER")
	password := flags.String("basic-auth-password", "", "password to require on this route, or $SWITCHBOARD_BASIC_AUTH_PASSWORD (prefer the variable, flags are visible in ps)")
	flags.StringVar(&config.Iface, "iface", "", "interface to listen on")
	flags.Parse(args)

	auth, err := basicAuth(*user, *password)
	if err != nil {
		slog.Error("Failed to read basic auth", "error", err)
		os.Exit(1)
	}

	host, p, err := net.SplitHostPort(*addr)
	if err != nil {
		slog.Error("Failed to parse address", "error", err, "addr", *addr)
		os.Exit(1)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		slog.Error("Failed to parse IP", "host", host)
		os.Exit(1)
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		slog.Error("Failed to parse port", "error", err, "port", p)
		os.Exit(1)
	}

	if err := client.Hookup(ctx, client.Service{
		Pattern: *pattern,
		IP:      ip,
		Port:    port,
		Auth:    auth,
	}); err != nil {
		slog.Error("Failed to hook up", "error", err)
		os.Exit(1)
	}
}

// basicAuth resolves the credential from flags, falling back to the
// environment. Neither set means an open route; one without the other is a
// mistake worth stopping for, since it would silently serve one.
func basicAuth(user, password string) (*operator.Auth, error) {
	if user == "" {
		user = os.Getenv("SWITCHBOARD_BASIC_AUTH_USER")
	}
	if password == "" {
		password = os.Getenv("SWITCHBOARD_BASIC_AUTH_PASSWORD")
	}
	switch {
	case user == "" && password == "":
		return nil, nil
	case user == "" || password == "":
		return nil, errors.New("basic auth needs both a username and a password")
	}
	return &operator.Auth{Username: user, Password: password}, nil
}
