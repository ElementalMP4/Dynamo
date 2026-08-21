package dns

import (
	"Dynamo/config"
	"Dynamo/store"
	"crypto/tls"
	"fmt"
	"strings"
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

type UpstreamDNSCache struct {
	msg     *dns.Msg
	expires time.Time
}

var upstreamCacheMutex sync.RWMutex
var upstreamCache = map[string]UpstreamDNSCache{}

func cacheKey(q dns.Question) string {
	return strings.ToLower(q.Name) + "|" + dns.TypeToString[q.Qtype]
}

func getCachedUpstream(q dns.Question) (*dns.Msg, bool) {
	upstreamCacheMutex.RLock()
	entry, ok := upstreamCache[cacheKey(q)]
	upstreamCacheMutex.RUnlock()

	if !ok {
		return nil, false
	}

	remaining := time.Until(entry.expires)
	if remaining <= 0 {
		return nil, false
	}

	reply := entry.msg.Copy()
	adjustTTLs(reply, uint32(remaining.Seconds()))
	return reply, true
}

func setCachedUpstream(q dns.Question, resp *dns.Msg) {
	minTTL := minAnswerTTL(resp)
	if minTTL == 0 {
		return
	}

	upstreamCacheMutex.Lock()
	upstreamCache[cacheKey(q)] = UpstreamDNSCache{
		msg:     resp.Copy(),
		expires: time.Now().Add(time.Duration(minTTL) * time.Second),
	}
	upstreamCacheMutex.Unlock()
}

func minAnswerTTL(resp *dns.Msg) uint32 {
	var min uint32
	first := true
	for _, rr := range resp.Answer {
		ttl := rr.Header().Ttl
		if first || ttl < min {
			min = ttl
			first = false
		}
	}
	if first {
		return 0
	}
	return min
}

func adjustTTLs(msg *dns.Msg, ttl uint32) {
	for _, rr := range msg.Answer {
		rr.Header().Ttl = ttl
	}
	for _, rr := range msg.Ns {
		rr.Header().Ttl = ttl
	}
	for _, rr := range msg.Extra {
		rr.Header().Ttl = ttl
	}
}

func cleanupExpiredCacheEntries() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		now := time.Now()
		upstreamCacheMutex.Lock()
		for k, v := range upstreamCache {
			if now.After(v.expires) {
				delete(upstreamCache, k)
			}
		}
		upstreamCacheMutex.Unlock()
	}
}

func isPrivateDomain(name, domain string) bool {
	name = strings.ToLower(dns.Fqdn(name))
	domain = strings.ToLower(dns.Fqdn(domain))
	return name == domain || strings.HasSuffix(name, "."+domain)
}

func SyncDNS() error {
	dnsStore = store.NewJSONStore[[]DNSRecord]("dns.json")
	go cleanupExpiredCacheEntries()
	return refreshRecordsFromStore(true)
}

func refreshRecordsFromStore(noisy bool) error {
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

		if noisy {
			log.Infof(loggingSource, "Registered %s record %s", rec.Type, rec.Name)
		}
	}

	return nil
}

func StartDNS() {
	dns.HandleFunc(".", handleDNSRequest)
	server := &dns.Server{Addr: config.GetConfig().DNS.ListenAddress, Net: "udp"}
	log.Okf(loggingSource, "DNS server running on %s", config.GetConfig().DNS.ListenAddress)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf(loggingSource, "Failed to start server: %v", err)
	}
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	dnsCfg := config.GetConfig().DNS
	domain := dns.Fqdn(dnsCfg.Domain)
	msg := dns.Msg{}
	msg.SetReply(r)

	for _, q := range r.Question {
		// If the query is not for our private domain, check the cache and then forward upstream
		if !isPrivateDomain(q.Name, domain) {
			if cached, ok := getCachedUpstream(q); ok {
				reply := new(dns.Msg)
				reply.SetReply(r)
				reply.Answer = cached.Answer
				reply.Ns = cached.Ns
				reply.Extra = cached.Extra
				reply.Rcode = cached.Rcode
				w.WriteMsg(reply)
				return
			}

			resp, err := forwardToUpstream(r)
			if err == nil && resp != nil {
				setCachedUpstream(q, resp)
				w.WriteMsg(resp)
				return
			}

			// Upstream failed and it's not our domain so there's nothing more we can do.
			w.WriteMsg(&msg)
			return
		}

		// Check our own list
		switch q.Qtype {
		case dns.TypeA:
			// Look for a direct A record
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

		// Send nothing if we find nothing
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
	dnsCfg := config.GetConfig().DNS

	var lastErr error
	for _, upstream := range dnsCfg.Upstream {
		c := dns.Client{
			Net:     "udp",
			Timeout: 2 * time.Second,
		}

		if upstream.UpstreamTLSServerName != "" {
			c.Net = "tcp-tls"
			c.TLSConfig = &tls.Config{
				ServerName: upstream.UpstreamTLSServerName,
				MinVersion: tls.VersionTLS12,
			}
		}
		resp, _, err := c.Exchange(req, upstream.DialAddress)

		if err != nil {
			lastErr = err
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

	err = refreshRecordsFromStore(false)
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

	if err := dnsStore.Save(filtered); err != nil {
		return nil, true, fmt.Errorf("Failed to write DNS records: %v", err)
	}

	err = refreshRecordsFromStore(false)
	if err != nil {
		return nil, true, fmt.Errorf("Failed to refresh DNS records: %v", err)
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

	err = refreshRecordsFromStore(false)
	if err != nil {
		return &patchedRecord, true, fmt.Errorf("Failed to refresh DNS records: %v", err)
	}

	return &patchedRecord, true, nil
}
