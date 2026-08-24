package client

import (
	"net"
	"os"
	"strings"
	"testing"

	"github.com/miekg/dns"
	"github.com/whytheplatypus/switchboard/config"
)

func testDoorbell(t *testing.T) (*doorbell, <-chan struct{}) {
	t.Helper()
	summons := make(chan struct{}, 1)
	d, err := newDoorbell(summons, net.ParseIP("10.0.0.4"), 8000)
	if err != nil {
		t.Fatal(err)
	}
	return d, summons
}

func rang(summons <-chan struct{}) bool {
	select {
	case <-summons:
		return true
	default:
		return false
	}
}

func TestDoorbellRingsForASummons(t *testing.T) {
	d, summons := testDoorbell(t)

	records := d.Records(dns.Question{
		Name:  config.HookupService + ".local.",
		Qtype: dns.TypePTR,
	})
	if len(records) == 0 {
		t.Fatal("a summons should still be answered")
	}
	if !rang(summons) {
		t.Fatal("a summons should ring the doorbell")
	}
}

// Everything here reaches Records on a busy network. None of it is an operator
// looking for hookups.
func TestDoorbellIgnoresTheRest(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		q    dns.Question
	}{
		{"service browser enumerating", dns.Question{Name: "_services._dns-sd._udp.local.", Qtype: dns.TypePTR}},
		{"somebody else's service", dns.Question{Name: "_http._tcp.local.", Qtype: dns.TypePTR}},
		{"a printer", dns.Question{Name: "_ipp._tcp.local.", Qtype: dns.TypePTR}},
		{"the operator's own service", dns.Question{Name: config.OperatorService + ".local.", Qtype: dns.TypePTR}},
		{"an address lookup for this host", dns.Question{Name: host + ".local.", Qtype: dns.TypeA}},
		{"resolving our instance, not looking for us", dns.Question{Name: host + "." + config.HookupService + ".local.", Qtype: dns.TypeSRV}},
		{"a name that merely starts the same", dns.Question{Name: config.HookupService + "-other.local.", Qtype: dns.TypePTR}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, summons := testDoorbell(t)
			d.Records(tt.q)
			if rang(summons) {
				t.Fatalf("%q rang the doorbell", tt.q.Name)
			}
		})
	}
}

// Being listed in the service browser directory is what invites every browser
// on the network to come and ask about us afterwards.
func TestDoorbellIsUnlisted(t *testing.T) {
	d, _ := testDoorbell(t)

	records := d.Records(dns.Question{Name: "_services._dns-sd._udp.local.", Qtype: dns.TypePTR})
	if len(records) != 0 {
		t.Fatalf("hookup answered the browser listing with %d records", len(records))
	}
}

// DNS names are case insensitive, so the filter has to be too.
func TestSummonedIgnoresCase(t *testing.T) {
	d, _ := testDoorbell(t)

	if !d.summoned(dns.Question{Name: strings.ToUpper(config.HookupService) + ".LOCAL.", Qtype: dns.TypePTR}) {
		t.Fatal("an upper case summons was not recognised")
	}
}
