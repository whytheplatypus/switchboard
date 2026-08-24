package config

import (
	"net"
	"testing"
)

// Left to itself the mdns library resolves the hostname, which on a box with a
// vpn or a hosts file entry answers with addresses nobody on this network can
// reach. Asking for an interface has to mean asking for its addresses.
func TestAddressesComeFromTheInterface(t *testing.T) {
	lo, err := net.InterfaceByName("lo")
	if err != nil {
		t.Skip("no loopback interface to test against")
	}

	before := Iface
	t.Cleanup(func() { Iface = before })
	Iface = lo.Name

	ips := Addresses()
	if len(ips) == 0 {
		t.Fatal("no addresses for the loopback interface")
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			t.Fatalf("announced %s, which is not on %s", ip, lo.Name)
		}
		if ip.IsLinkLocalUnicast() {
			t.Fatalf("announced link local %s, which cannot be put in a url", ip)
		}
	}
}

// No interface asked for means the library's hostname lookup, unchanged.
func TestAddressesAreEmptyWithoutAnInterface(t *testing.T) {
	before := Iface
	t.Cleanup(func() { Iface = before })
	Iface = ""

	if ips := Addresses(); ips != nil {
		t.Fatalf("got %v, want nil", ips)
	}
}
