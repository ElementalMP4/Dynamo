package dns

import (
	"Dynamo/config"
	"Dynamo/store"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/ElementalMP4/loggerino/log"
	"github.com/google/uuid"
)

const dnsmasqConf = "/etc/dnsmasq.conf"
const loggingSource string = "dns-router"

var dnsStore *store.JSONStore[[]DNSRecord]
var dnsMutationMu sync.Mutex

func SyncDns() error {
	configFile := config.GetConfig().CreateDnsmasqConfigFile()

	dnsStore = store.NewJSONStore[[]DNSRecord]("dns.json")
	log.Info("dns-sync", "Syncing DNS")

	records, err := dnsStore.Load()
	if err != nil {
		return err
	}

	idsMigrated := false
	for i := range records {
		if _, err := uuid.Parse(records[i].ID); err == nil {
			continue
		}

		records[i].ID = uuid.NewString()
		idsMigrated = true
	}

	if idsMigrated {
		log.Warn("dns-sync", "Migrated legacy DNS IDs to UUIDs")
	}

	_, confErr := os.Stat(dnsmasqConf)
	_, oldErr := os.Stat(dnsmasqConf + ".old")
	if confErr == nil && os.IsNotExist(oldErr) {
		if err := os.Rename(dnsmasqConf, dnsmasqConf+".old"); err != nil {
			return fmt.Errorf("failed to back up dnsmasq.conf: %v", err)
		}
	}

	if err := os.WriteFile(dnsmasqConf, []byte(configFile), 0644); err != nil {
		return fmt.Errorf("failed to write dnsmasq.conf: %v", err)
	}

	err = writeRecords(records)
	if err != nil {
		return err
	}

	log.Okf("dns-sync", "DNS sync completed successfully")

	return nil
}

func EnsureWritable() error {
	f, err := os.OpenFile(config.GetConfig().System.DnsmasqFile, os.O_RDWR|os.O_CREATE, 0644)
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
	err := dnsStore.Save(records)
	if err != nil {
		return fmt.Errorf("failed to save DNS state: %v", err)
	}

	var out strings.Builder
	var hosts strings.Builder

	out.WriteString("# Managed by Dynamo - Do not edit manually! Edits will be overwritten\n\n")

	for _, r := range records {
		out.WriteString(fmt.Sprintf("# ttl=%d\n", r.TTL))

		switch r.Type {
		case "A":
			out.WriteString(fmt.Sprintf("address=/%s/%s\n", r.Name, r.IPv4))
			hosts.WriteString(fmt.Sprintf("%s %s\n", r.IPv4, r.Name))
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

	if err := os.WriteFile(config.GetConfig().System.DnsmasqFile, []byte(out.String()), 0644); err != nil {
		return err
	}

	if err := os.WriteFile(config.GetConfig().System.DataDirectory+"/hosts", []byte(hosts.String()), 0644); err != nil {
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
