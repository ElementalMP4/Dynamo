package dns

import (
	"Dynamo/config"
	"Dynamo/store"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ElementalMP4/loggerino/log"
)

const loggingSource string = "dns-router"

var dnsStore *store.JSONStore[[]DNSRecord]

func SyncDns() error {
	dnsStore = store.NewJSONStore[[]DNSRecord]("dns.json")
	log.Info("dns-sync", "Syncing DNS")

	records, err := dnsStore.Load()
	if err != nil {
		return err
	}

	err = writeRecords(records)
	if err != nil {
		return err
	}

	log.Okf("dns-sync", "DNS sync completed successfully")

	return nil
}

func EnsureWritable() error {
	f, err := os.OpenFile(config.GetConfig().DnsmasqFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	f.Close()

	return nil
}

func reloadDnsmasq() error {
	cmd := exec.Command("systemctl", "restart", "dnsmasq")
	return cmd.Run()
}

func writeRecords(records []DNSRecord) error {
	var out strings.Builder

	out.WriteString("# Managed by Dynamo - Do not edit manually! Edits will be overwritten\n\n")

	for _, r := range records {
		out.WriteString(fmt.Sprintf("# ttl=%d\n", r.TTL))

		switch r.Type {
		case "A":
			out.WriteString(fmt.Sprintf("address=/%s/%s\n", r.Name, r.IPv4))
		case "CNAME":
			out.WriteString(fmt.Sprintf("cname=%s,%s\n", r.Name, r.CNAME))
		case "TXT":
			for _, t := range r.TXT {
				out.WriteString(fmt.Sprintf("txt-record=%s,\"%s\"\n", r.Name, t))
			}
		case "SRV":
			out.WriteString(fmt.Sprintf(
				"srv-host=%s,%s,%d,%d,%d\n",
				r.Name, r.Target, r.Port, r.Priority, r.Weight,
			))
		}

		out.WriteString("\n")
		log.Infof("dns", "Created %s record for %s", r.Type, r.Name)
	}

	if err := os.WriteFile(config.GetConfig().DnsmasqFile, []byte(out.String()), 0644); err != nil {
		return err
	}

	return reloadDnsmasq()
}

func CountDNSRecords() (map[string]int, error) {
	records, err := dnsStore.Load()
	if err != nil {
		return nil, err
	}

	counts := map[string]int{
		"A":     0,
		"CNAME": 0,
		"TXT":   0,
		"SRV":   0,
	}

	for _, record := range records {
		counts[record.Type]++
	}

	return counts, nil
}
