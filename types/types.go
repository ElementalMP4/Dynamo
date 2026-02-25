package types

import (
	"fmt"
	"strconv"
)

type DNSRecord struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	TTL   int    `json:"ttl"`
	Class string `json:"class"`

	IPv4  string   `json:"ipv4,omitempty"`
	CNAME string   `json:"cname,omitempty"`
	TXT   []string `json:"txt,omitempty"`

	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	Port     int    `json:"port,omitempty"`
	Target   string `json:"target,omitempty"`
}

type Config struct {
	ServerAddress string `json:"serverBind"`
	CertsDir      string `json:"certsDir"`
	DnsmasqFile   string `json:"dnsmasqConfigFile"`
}

type CircuitType string

const (
	CircuitRedirect CircuitType = "REDIRECT"
	CircuitDNAT     CircuitType = "DNAT"
)

type RoutingCircuit struct {
	ID          int64       `db:"id" json:"id"`
	Name        string      `db:"name" json:"name"`
	Type        CircuitType `db:"type" json:"type"`
	Enabled     bool        `db:"enabled" json:"enabled"`
	InInterface *string     `db:"in_interface" json:"in_interface,omitempty"`

	Protocol string `db:"protocol" json:"protocol"`
	SrcPort  int    `db:"src_port" json:"src_port"`

	RedirectPort *int `db:"redirect_port" json:"redirect_port,omitempty"`

	DestIP   *string `db:"dest_ip" json:"dest_ip,omitempty"`
	DestPort *int    `db:"dest_port" json:"dest_port,omitempty"`
}

func (c RoutingCircuit) IPTablesRules() [][]string {
	var rules [][]string
	comment := fmt.Sprintf("dynamo-circuit-%d", c.ID)

	switch c.Type {

	case CircuitRedirect:
		args := []string{
			"-t", "nat",
			"-A", "PREROUTING",
			"-p", c.Protocol,
			"--dport", strconv.Itoa(c.SrcPort),
		}

		if c.InInterface != nil {
			args = append(args, "-i", *c.InInterface)
		}

		args = append(args,
			"-m", "comment",
			"--comment", comment,
			"-j", "REDIRECT",
			"--to-port", strconv.Itoa(*c.RedirectPort),
		)

		rules = append(rules, args)

	case CircuitDNAT:
		// PREROUTING (DNAT)
		rules = append(rules, []string{
			"-t", "nat",
			"-A", "PREROUTING",
			"-p", c.Protocol,
			"--dport", strconv.Itoa(c.SrcPort),
			"-m", "comment",
			"--comment", comment,
			"-j", "DNAT",
			"--to-destination", fmt.Sprintf("%s:%d", *c.DestIP, *c.DestPort),
		})

		// FORWARD (new + established)
		rules = append(rules, []string{
			"-A", "FORWARD",
			"-p", c.Protocol,
			"-d", *c.DestIP,
			"--dport", strconv.Itoa(*c.DestPort),
			"-m", "conntrack",
			"--ctstate", "NEW,ESTABLISHED,RELATED",
			"-m", "comment",
			"--comment", comment,
			"-j", "ACCEPT",
		})

		// RETURN traffic
		rules = append(rules, []string{
			"-A", "FORWARD",
			"-p", c.Protocol,
			"-s", *c.DestIP,
			"--sport", strconv.Itoa(*c.DestPort),
			"-m", "conntrack",
			"--ctstate", "ESTABLISHED,RELATED",
			"-m", "comment",
			"--comment", comment,
			"-j", "ACCEPT",
		})

		// POSTROUTING (SNAT)
		rules = append(rules, []string{
			"-t", "nat",
			"-A", "POSTROUTING",
			"-p", c.Protocol,
			"-d", *c.DestIP,
			"-m", "comment",
			"--comment", comment,
			"-j", "MASQUERADE",
		})
	}

	return rules
}

type CertDir struct {
	Name  string   `json:"name,omitempty"`
	Files []string `json:"files"`
}

type CertListing map[string][]CertDir
