package client

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
)

func Hookup(pattern string, port int) *mdns.Server {
	// Setup our service export
	host, _ := os.Hostname()
	info := []string{pattern}
	service, _ := mdns.NewMDNSService(
		host,
		fmt.Sprintf("%s", config.ServiceName),
		"",
		"",
		port,
		nil,
		info,
	)
	conf := &mdns.Config{Zone: service}
	if config.Iface != "" {
		if iface, err := net.InterfaceByName(config.Iface); err == nil {
			slog.Info("Using interface provided", "interface", iface.Name)
			conf.Iface = iface
		} else {
			slog.Error("failed to get interface", "error", err)
		}
	}

	// Create the mDNS server, defer shutdown
	server, _ := mdns.NewServer(conf)
	return server
}
