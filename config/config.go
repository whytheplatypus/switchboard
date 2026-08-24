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
	// Retry is how soon a hookup tries again after a pass that reached no
	// operator at all. It doubles as the floor on how fast anything ringing
	// the doorbell can drive a hookup.
	Retry = 5 * time.Second
	// Reach is how long one call to an operator may take before it is given
	// up on. Without it a single unreachable operator that still accepts
	// connections stalls the heartbeat indefinitely.
	Reach = 5 * time.Second
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
	// Pinning to an interface that cannot carry multicast is silent otherwise:
	// nothing is ever discovered and nothing says why.
	if iface.Flags&net.FlagMulticast == 0 {
		slog.Warn("interface does not support multicast, mdns will find nothing", "interface", iface.Name)
	}
	if iface.Flags&net.FlagUp == 0 {
		slog.Warn("interface is down", "interface", iface.Name)
	}
	slog.Info("Using interface provided", "interface", iface.Name)
	return iface
}

// Addresses are the addresses to announce over mDNS. Left to itself the mdns
// library resolves the hostname, which on a box with a vpn, a container
// bridge, or a hosts file entry answers with addresses no one on this network
// can reach, and answers with a different one each time. Pinning an interface
// pins the addresses announced on it.
//
// A nil result means fall back to the library's hostname lookup, which is what
// happens when no interface was asked for.
func Addresses() []net.IP {
	iface := Interface()
	if iface == nil {
		return nil
	}
	addrs, err := iface.Addrs()
	if err != nil {
		slog.Error("failed to read interface addresses", "error", err, "interface", iface.Name)
		return nil
	}
	ips := []net.IP{}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		// A link local address needs a zone to be usable and cannot be put in
		// a url, so announcing one only offers an address that will not work.
		if ipnet.IP.IsLinkLocalUnicast() || ipnet.IP.IsLinkLocalMulticast() {
			continue
		}
		ips = append(ips, ipnet.IP)
	}
	if len(ips) == 0 {
		slog.Error("interface has no usable addresses", "interface", iface.Name)
		return nil
	}
	slog.Info("Announcing addresses", "interface", iface.Name, "addresses", ips)
	return ips
}
