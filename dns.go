package main

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/miekg/dns"
)

type DNSHandler struct {
	sync.RWMutex

	zone string

	svcMap map[string]string
}

func NewDNSHandler(zone string) *DNSHandler {
	return &DNSHandler{
		zone:   zone,
		svcMap: make(map[string]string),
	}
}

func (h *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg.Authoritative = true
		domain := msg.Question[0].Name

		h.RLock()
		address, ok := h.svcMap[domain]
		h.RUnlock()
		if !ok {
			msg.SetRcode(r, dns.RcodeNameError)
			break
		}

		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP(address),
		})

		RecordLastServed.WithLabelValues(domain, address).SetToCurrentTime()
	}
	w.WriteMsg(&msg)
}

func (h *DNSHandler) UpdateRecord(service string, records []string) {
	h.Lock()
	defer h.Unlock()

	rec := fmt.Sprintf("%s.%s.", service, h.zone)
	cur := h.svcMap[rec]

	for _, record := range records {
		if cur == record {
			return
		}
	}

	newRecord := records[0]
	log.Printf("Updating service map record %s (%s)", service, newRecord)
	h.svcMap[rec] = newRecord
}
