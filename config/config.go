package config

import (
	"log/slog"
	"net"
	"time"
)

// The two mDNS names. Each is used for exactly one thing: operators announce
// where their registration API lives, hookups answer the door.
const (
	OperatorService = "_switchboard-operator"
	HookupService   = "_switchboard-hookup"
)

// Both sides have to agree on these, so they live here rather than in flags.
const (
	// Lease is how long a registration routes without being refreshed.
	Lease = 90 * time.Second
	// Heartbeat is how often a hookup refreshes its registration. It must be
	// comfortably shorter than Lease so a single lost packet costs nothing.
	Heartbeat = 30 * time.Second
	// Summon is how often an operator asks the network's hookups to register.
	Summon = 60 * time.Second
)

var Iface string

// Interface resolves the -iface flag for the mDNS listeners. A nil result
// means the system default interface, which is also what a bad name falls
// back to.
func Interface() *net.Interface {
	if Iface == "" {
		return nil
	}
	iface, err := net.InterfaceByName(Iface)
	if err != nil {
		slog.Error("failed to get interface", "error", err, "interface", Iface)
		return nil
	}
	slog.Info("Using interface provided", "interface", iface.Name)
	return iface
}
