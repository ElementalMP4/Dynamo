package dns

import (
	"Dynamo/config"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const loggingSource string = "dns-router"

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

func loadRecords() ([]DNSRecord, error) {
	file, err := os.Open(config.GetConfig().DnsmasqFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []DNSRecord
	scanner := bufio.NewScanner(file)
	id := 1
	ttl := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "# ttl=") {
			ttl, _ = strconv.Atoi(strings.TrimPrefix(line, "# ttl="))
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rec := DNSRecord{
			ID:  id,
			TTL: ttl,
		}

		switch {
		case strings.HasPrefix(line, "address=/"):
			rec.Type = "A"
			parts := strings.Split(strings.TrimPrefix(line, "address=/"), "/")
			rec.Name = parts[0]
			rec.IPv4 = parts[1]

		case strings.HasPrefix(line, "cname="):
			rec.Type = "CNAME"
			parts := strings.Split(strings.TrimPrefix(line, "cname="), ",")
			rec.Name = parts[0]
			rec.CNAME = parts[1]

		case strings.HasPrefix(line, "txt-record="):
			rec.Type = "TXT"
			parts := strings.SplitN(strings.TrimPrefix(line, "txt-record="), ",", 2)
			rec.Name = parts[0]
			rec.TXT = []string{strings.Trim(parts[1], `"`)}

		case strings.HasPrefix(line, "srv-host="):
			rec.Type = "SRV"
			parts := strings.Split(strings.TrimPrefix(line, "srv-host="), ",")
			rec.Name = parts[0]
			rec.Target = parts[1]
			port, _ := strconv.Atoi(parts[2])
			rec.Port = port
			priority, _ := strconv.Atoi(parts[3])
			rec.Priority = priority
			weight, _ := strconv.Atoi(parts[4])
			rec.Weight = weight
		default:
			continue
		}

		records = append(records, rec)
		id++
	}

	return records, nil
}

func writeRecords(records []DNSRecord) error {
	var out strings.Builder

	out.WriteString("# Managed by Dynamo\n\n")

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
	}

	if err := os.WriteFile(config.GetConfig().DnsmasqFile, []byte(out.String()), 0644); err != nil {
		return err
	}

	return reloadDnsmasq()
}

func CountDNSRecords() (map[string]int, error) {
	records, err := loadRecords()
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
