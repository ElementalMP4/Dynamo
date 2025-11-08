package dns

import (
	"TinyVTM/config"
	"TinyVTM/db"
	"TinyVTM/log"
	"TinyVTM/types"
	"fmt"
	"sync"

	"github.com/miekg/dns"
)

var mu sync.RWMutex

var localA = map[string]types.DnsRecord{}
var localCNAME = map[string]types.DnsRecord{}
var localTXT = map[string]types.DnsRecord{}
var localSRV = map[string]types.DnsRecord{}
var upstream = "8.8.8.8:53"

func StartDNS() {
	// Load records from the database into memory.
	if records, err := db.GetAllDnsRecords(); err != nil {
		log.LogError(fmt.Sprintf("failed to load DNS records from DB: %v", err))
	} else {
		mu.Lock()
		for _, rec := range records {
			switch rec.Type {
			case "A":
				if rec.Name != "" {
					localA[rec.NameTrailingDot()] = rec
				}
			case "CNAME":
				if rec.Name != "" {
					localCNAME[rec.NameTrailingDot()] = rec
				}
			case "TXT":
				if rec.Name != "" {
					localTXT[rec.NameTrailingDot()] = rec
				}
			case "SRV":
				if rec.Name != "" {
					localSRV[rec.NameTrailingDot()] = rec
				}
			}
		}
		mu.Unlock()
	}

	dns.HandleFunc(".", handleDNSRequest)
	server := &dns.Server{Addr: config.GetConfig().DnsAddress, Net: "udp"}
	log.LogInfo("Starting DNS server on " + config.GetConfig().DnsAddress)
	err := server.ListenAndServe()
	if err != nil {
		log.LogFatal(fmt.Sprintf("Failed to start server: %v", err))
	}
}

func AddOrUpdateRecord(rec types.DnsRecord) {
	if rec.Name == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	switch rec.Type {
	case "A":
		localA[rec.Name] = rec
	case "CNAME":
		localCNAME[rec.Name] = rec
	case "TXT":
		localTXT[rec.Name] = rec
	case "SRV":
		localSRV[rec.Name] = rec
	}
}

func DeleteRecordByNameType(name, rtype string) {
	if name == "" || rtype == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	switch rtype {
	case "A":
		delete(localA, name)
	case "CNAME":
		delete(localCNAME, name)
	case "TXT":
		delete(localTXT, name)
	case "SRV":
		delete(localSRV, name)
	}
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)

	for _, q := range r.Question {
		log.LogDuration(fmt.Sprintf("Resolve %s (%s %s)", q.Name, dns.Type(q.Qtype), q.Name), func() error {
			switch q.Qtype {
			case dns.TypeA:
				if rec, ok := localA[q.Name]; ok {
					ip := rec.IPv4.String()
					rr, _ := dns.NewRR(q.Name + " IN A " + ip)
					msg.Answer = append(msg.Answer, rr)
					return w.WriteMsg(&msg)
				}

			case dns.TypeCNAME:
				if rec, ok := localCNAME[q.Name]; ok {
					rr, _ := dns.NewRR(q.Name + " IN CNAME " + rec.CNAME)
					msg.Answer = append(msg.Answer, rr)
					return w.WriteMsg(&msg)
				}

			case dns.TypeTXT:
				if rec, ok := localTXT[q.Name]; ok {
					rr, _ := dns.NewRR(q.Name + " IN TXT \"" + rec.ToTxtResponse() + "\"")
					msg.Answer = append(msg.Answer, rr)
					return w.WriteMsg(&msg)
				}

			case dns.TypeSRV:
				if rec, ok := localSRV[q.Name]; ok {
					rr, _ := dns.NewRR(q.Name + " IN SRV " + rec.ToSrvResponse())
					msg.Answer = append(msg.Answer, rr)
					return w.WriteMsg(&msg)
				}
			}

			// Send upstream if we don't have a record
			resp, err := forwardToUpstream(r)
			if err == nil && resp != nil {
				return w.WriteMsg(resp)
			}
			// Send nothing if upstream DNS fails
			return w.WriteMsg(&msg)
		})
	}
}

func forwardToUpstream(req *dns.Msg) (*dns.Msg, error) {
	c := dns.Client{Net: "udp"}
	resp, _, err := c.Exchange(req, upstream)
	if err != nil {
		log.LogError(fmt.Sprintf("Upstream resolution failed: %v", err))
		return nil, err
	}
	return resp, nil
}
