package dns

import (
	"Dynamo/config"
	"Dynamo/store"
	"fmt"
	"sync"
	"time"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/google/uuid"
	"github.com/miekg/dns"
)

const loggingSource string = "dns"

var dnsStore *store.JSONStore[[]DNSRecord]
var dnsMutationMutex sync.RWMutex

var localA = map[string]DNSRecord{}
var localCNAME = map[string]DNSRecord{}
var localTXT = map[string]DNSRecord{}
var localSRV = map[string]DNSRecord{}

func SyncDNS() error {
	dnsStore = store.NewJSONStore[[]DNSRecord]("dns.json")
	return refreshRecordsFromStore()
}

func refreshRecordsFromStore() error {
	log.Info("dns-sync", "Syncing DNS")

	records, err := dnsStore.Load()
	if err != nil {
		return err
	}

	dnsMutationMutex.Lock()
	defer dnsMutationMutex.Unlock()

	localA = make(map[string]DNSRecord)
	localCNAME = make(map[string]DNSRecord)
	localTXT = make(map[string]DNSRecord)
	localSRV = make(map[string]DNSRecord)

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
		log.Infof(loggingSource, "Registered %s record %s", rec.Type, rec.Name)
	}

	return nil
}

func StartDNS() {
	dns.HandleFunc(".", handleDNSRequest)
	server := &dns.Server{Addr: config.GetConfig().DnsmasqConfig.ListenAddress, Net: "udp"}
	log.Okf(loggingSource, "DNS server running on %s", config.GetConfig().DnsmasqConfig.ListenAddress)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf(loggingSource, "Failed to start server: %v", err)
	}
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)

	for _, q := range r.Question {
		log.Infof(loggingSource, "Resolving %s", q.Name)

		switch q.Qtype {
		case dns.TypeA:
			// First, look for a direct A record
			if rec, ok := localA[q.Name]; ok {
				rr, err := dns.NewRR(q.Name + " IN A " + rec.IPv4)
				if err == nil {
					msg.Answer = append(msg.Answer, rr)
					w.WriteMsg(&msg)
					return
				}

				log.Errorf(loggingSource, "Failed to create A record for %s: %v", q.Name, err)
			}

			// No direct A record, so check for a CNAME
			if cnameRec, ok := localCNAME[q.Name]; ok {
				appendCNAMEAndA(&msg, q.Name, cnameRec)
				w.WriteMsg(&msg)
				return
			}

		case dns.TypeCNAME:
			if cnameRec, ok := localCNAME[q.Name]; ok {
				appendCNAMEAndA(&msg, q.Name, cnameRec)
				w.WriteMsg(&msg)
				return
			}

		case dns.TypeTXT:
			if rec, ok := localTXT[q.Name]; ok {
				rr, err := dns.NewRR(q.Name + " IN TXT \"" + rec.ToTxtResponse() + "\"")
				if err == nil {
					msg.Answer = append(msg.Answer, rr)
					w.WriteMsg(&msg)
					return
				}

				log.Errorf(loggingSource, "Failed to create TXT record for %s: %v", q.Name, err)
			}

		case dns.TypeSRV:
			if rec, ok := localSRV[q.Name]; ok {
				rr, err := dns.NewRR(q.Name + " IN SRV " + rec.ToSrvResponse())
				if err == nil {
					msg.Answer = append(msg.Answer, rr)
					w.WriteMsg(&msg)
					return
				}

				log.Errorf(loggingSource, "Failed to create SRV record for %s: %v", q.Name, err)
			}
		}

		// Send upstream if we don't have a local record
		resp, err := forwardToUpstream(r)
		if err == nil && resp != nil {
			w.WriteMsg(resp)
			return
		}

		// Send nothing if upstream DNS fails
		w.WriteMsg(&msg)
	}
}

func appendCNAMEAndA(msg *dns.Msg, name string, cnameRec DNSRecord) {
	target := dns.Fqdn(cnameRec.CNAME)

	cnameRR, err := dns.NewRR(name + " IN CNAME " + target)
	if err != nil {
		log.Errorf(loggingSource, "Failed to create CNAME record for %s: %v", name, err)
		return
	}

	msg.Answer = append(msg.Answer, cnameRR)

	if aRec, ok := localA[target]; ok {
		aRR, err := dns.NewRR(target + " IN A " + aRec.IPv4)
		if err != nil {
			log.Errorf(loggingSource, "Failed to create A record for CNAME target %s: %v", target, err)
			return
		}

		msg.Answer = append(msg.Answer, aRR)
	}
}

func forwardToUpstream(req *dns.Msg) (*dns.Msg, error) {
	c := dns.Client{
		Net:     "udp",
		Timeout: 2 * time.Second,
	}

	var lastErr error

	for _, upstream := range config.GetConfig().DnsmasqConfig.Upstream {
		resp, _, err := c.Exchange(req, upstream)

		if err != nil {
			lastErr = err
			log.Errorf(loggingSource, "Upstream %s failed: %v", upstream, err)
			continue
		}

		switch resp.Rcode {
		case dns.RcodeSuccess, dns.RcodeNameError:
			return resp, nil

		case dns.RcodeServerFailure, dns.RcodeRefused:
			lastErr = fmt.Errorf("%s returned %s", upstream, dns.RcodeToString[resp.Rcode])
			continue

		default:
			return resp, nil
		}
	}

	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}

func CountDNSRecords() map[string]int {
	return map[string]int{
		"A":     len(localA),
		"CNAME": len(localCNAME),
		"TXT":   len(localTXT),
		"SRV":   len(localSRV),
	}
}

func createNewRecord(record DNSRecord) error {
	record.ID = uuid.NewString()

	records, err := dnsStore.Load()
	if err != nil {
		return fmt.Errorf("Failed to load DNS records: %v", err)
	}

	records = append(records, record)
	if err := dnsStore.Save(records); err != nil {
		return fmt.Errorf("Failed to write DNS records: %v", err)
	}

	err = refreshRecordsFromStore()
	if err != nil {
		return fmt.Errorf("Failed to refresh DNS records: %v", err)
	}

	return nil
}

func deleteRecord(id string) (*DNSRecord, bool, error) {
	records, err := dnsStore.Load()
	if err != nil {
		return nil, false, fmt.Errorf("Failed to load DNS records: %v", err)
	}

	var toBeDeleted DNSRecord
	var filtered = []DNSRecord{}

	found := false

	for _, record := range records {
		if record.ID == id {
			found = true
			toBeDeleted = record
		} else {
			filtered = append(filtered, record)
		}
	}

	if !found {
		return nil, false, nil
	}

	if err := dnsStore.Save(records); err != nil {
		return &toBeDeleted, true, fmt.Errorf("Failed to write DNS records: %v", err)
	}

	err = refreshRecordsFromStore()
	if err != nil {
		return &toBeDeleted, true, fmt.Errorf("Failed to refresh DNS records: %v", err)
	}

	return &toBeDeleted, true, nil
}

func patchRecord(id string, patch DNSRecordPatch) (*DNSRecord, bool, error) {
	records, err := dnsStore.Load()
	if err != nil {
		return nil, false, fmt.Errorf("Failed to load DNS records: %v", err)
	}

	found := false
	var patchedRecord DNSRecord

	for i := range records {
		if records[i].ID != id {
			continue
		}
		found = true

		if patch.Name != nil {
			records[i].Name = *patch.Name
		}
		if patch.Type != nil {
			records[i].Type = *patch.Type
		}
		if patch.TTL != nil {
			records[i].TTL = *patch.TTL
		}
		if patch.IPv4 != nil {
			records[i].IPv4 = *patch.IPv4
		}
		if patch.CNAME != nil {
			records[i].CNAME = *patch.CNAME
		}
		if patch.TXT != nil {
			records[i].TXT = *patch.TXT
		}
		if patch.Target != nil {
			records[i].Target = *patch.Target
		}
		if patch.Port != nil {
			records[i].Port = *patch.Port
		}
		if patch.Priority != nil {
			records[i].Priority = *patch.Priority
		}
		if patch.Weight != nil {
			records[i].Weight = *patch.Weight
		}

		patchedRecord = records[i]
		break
	}

	if !found {
		return nil, false, nil
	}

	if err := dnsStore.Save(records); err != nil {
		return &patchedRecord, true, fmt.Errorf("Failed to write DNS records: %v", err)
	}

	err = refreshRecordsFromStore()
	if err != nil {
		return &patchedRecord, true, fmt.Errorf("Failed to refresh DNS records: %v", err)
	}

	return &patchedRecord, true, nil
}
