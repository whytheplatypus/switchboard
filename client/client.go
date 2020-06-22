package client

import (
	"fmt"
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

	// Create the mDNS server, defer shutdown
	server, _ := mdns.NewServer(&mdns.Config{Zone: service})
	return server
}
