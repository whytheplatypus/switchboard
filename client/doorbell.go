package client

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
	"github.com/whytheplatypus/switchboard/config"
)

// enumeration is the address service browsers ask for when they want to know
// what kinds of service exist on a network.
const enumeration = "_services._dns-sd._udp."

// A doorbell is an mDNS zone that reports being asked about. The server hands
// it every question on the network, so it has to be particular about which
// ones mean an operator is looking for hookups.
type doorbell struct {
	*mdns.MDNSService
	service  string
	instance string
	summons  chan<- struct{}
}

func newDoorbell(summons chan<- struct{}, ip net.IP, port int) (*doorbell, error) {
	instance, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	service, err := mdns.NewMDNSService(instance, config.HookupService, "", "", port, []net.IP{ip}, nil)
	if err != nil {
		return nil, err
	}
	return &doorbell{
		MDNSService: service,
		service:     strings.ToLower(fmt.Sprintf("%s.local.", config.HookupService)),
		instance:    strings.ToLower(fmt.Sprintf("%s.%s.local.", instance, config.HookupService)),
		summons:     summons,
	}, nil
}

func (d *doorbell) Records(q dns.Question) []dns.RR {
	// Stay out of the service browser listing. A hookup is not something
	// anyone browses for, and being listed is precisely what sends every
	// browser on the network -- a desktop with a file manager open, a phone,
	// a home automation box -- around to ask about it afterwards. Unlisted,
	// the only things that ask are the ones that already knew the name.
	if strings.HasPrefix(strings.ToLower(q.Name), enumeration) {
		return nil
	}

	records := d.MDNSService.Records(q)
	// Ring only for a question we actually answered, and only for the kind of
	// question a summons is. An address lookup for this host is not a summons.
	if len(records) > 0 && d.summoned(q) {
		select {
		case d.summons <- struct{}{}:
		default: // one pending summons is as good as ten
		}
	}
	return records
}

func (d *doorbell) summoned(q dns.Question) bool {
	if q.Qtype != dns.TypePTR && q.Qtype != dns.TypeANY {
		return false
	}
	name := strings.ToLower(q.Name)
	return name == d.service || name == d.instance
}
