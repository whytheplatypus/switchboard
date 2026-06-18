package client

import (
	"crypto/md5"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/hashicorp/mdns"
	"github.com/whytheplatypus/switchboard/config"
)

func Hookup(pattern string, port int, ips ...net.IP) *mdns.Server {
	// Setup our service export
	instance, _ := os.Hostname()
	if len(ips) == 0 {
		slog.Error("No IP addresses provided")
		return nil
	}
	id := fmt.Sprintf("%s-%s-%d", pattern, ips[0].String(), port)
	hash := fmt.Sprintf("%x", md5.Sum([]byte(id)))
	info := []string{pattern, hash}
	service, err := mdns.NewMDNSService(
		instance,
		fmt.Sprintf("%s", config.ServiceName),
		"",
		"",
		port,
		ips,
		info,
	)
	if err != nil {
		slog.Error("failed to create service", "error", err)
		return nil
	}
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
	server, err := mdns.NewServer(conf)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		return nil
	}
	return server
}
