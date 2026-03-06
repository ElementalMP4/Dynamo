package types

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const iptablesTimeout = 10 * time.Second

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
	CircuitLocal    CircuitType = "LOCAL"
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

	case CircuitLocal:
		// LOCAL traffic (only to/from loopback)
		args := []string{
			"-t", "nat",
			"-A", "OUTPUT",
			"-o", "lo",
			"-p", c.Protocol,
			"-m", c.Protocol,
			"--dport", strconv.Itoa(c.SrcPort),
			"-m", "comment",
			"--comment", comment,
			"-j", "REDIRECT",
			"--to-ports", strconv.Itoa(*c.RedirectPort),
		}
		rules = append(rules, args)
	}

	return rules
}

func (c *RoutingCircuit) expandProtocols() []RoutingCircuit {
	if c.Protocol != "both" {
		return []RoutingCircuit{*c}
	}

	tcp := c
	tcp.Protocol = "tcp"

	udp := c
	udp.Protocol = "udp"

	return []RoutingCircuit{*tcp, *udp}
}

func (c *RoutingCircuit) EnableCircuit() error {
	log.Printf("Enabling circuit %s", c.Name)

	enabled, err := c.CircuitIsEnabled()
	if err != nil {
		return fmt.Errorf("failed to list firewall rules: %w", err)
	}

	if enabled {
		log.Printf("Circuit %s is already enabled", c.Name)
		return nil
	}

	for _, expanded := range c.expandProtocols() { // Loop over TCP & UDP
		for _, rule := range expanded.IPTablesRules() { // Loop over individual iptables commands

			ctx, cancel := context.WithTimeout(context.Background(), iptablesTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, "iptables", rule...)

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("error whilst calling iptables: %v", err)
			}
		}
	}
	log.Printf("Enabled circuit %s successfully", c.Name)
	return nil
}

func (c *RoutingCircuit) dropCircuitConntrack() error {
	var errs []error

	protos := []string{c.Protocol}
	if c.Protocol == "both" {
		protos = []string{"tcp", "udp"}
	}

	for _, proto := range protos {
		cmd := exec.Command(
			"conntrack",
			"-D",
			"-p", proto,
			"--dport", strconv.Itoa(c.SrcPort),
		)

		if err := cmd.Run(); err != nil {
			// conntrack returns exit 1 if nothing matched, so we can ignore this
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				continue
			}
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("conntrack errors: %v", errs)
	}

	return nil
}

func (c *RoutingCircuit) CircuitIsEnabled() (bool, error) {
	comment := fmt.Sprintf("dynamo-circuit-%d", c.ID)

	match1 := fmt.Sprintf("--comment %s ", comment)  // Space after %s is required
	match2 := fmt.Sprintf(`--comment "%s"`, comment) // Also look for quoted comments

	saveCmd := exec.Command("iptables-save")
	saveOut, err := saveCmd.Output()
	if err != nil {
		return false, fmt.Errorf("iptables-save failed: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(saveOut))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, match1) || strings.Contains(line, match2) {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("failed reading iptables-save output: %w", err)
	}

	return false, nil
}

func (c *RoutingCircuit) deleteCircuitRulesByComment() error {
	comment := fmt.Sprintf("dynamo-circuit-%d", c.ID)

	match1 := fmt.Sprintf("--comment %s ", comment)  // Space after %s is required
	match2 := fmt.Sprintf(`--comment "%s"`, comment) // Also look for quoted comments

	saveCmd := exec.Command("iptables-save")
	saveOut, err := saveCmd.Output()
	if err != nil {
		return fmt.Errorf("iptables-save failed: %w", err)
	}

	var filtered bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(saveOut))
	for scanner.Scan() {
		line := scanner.Text()

		// If we found the comment, don't add the rule to the filtered output
		if strings.Contains(line, match1) || strings.Contains(line, match2) {
			continue
		}

		filtered.WriteString(line)
		filtered.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed reading iptables-save output: %w", err)
	}

	restoreCmd := exec.Command("iptables-restore")
	restoreCmd.Stdin = &filtered
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr

	if err := restoreCmd.Run(); err != nil {
		return fmt.Errorf("iptables-restore failed: %w", err)
	}

	return nil
}

func (c *RoutingCircuit) DisableCircuit() error {
	log.Printf("Disabling circuit %s", c.Name)

	enabled, err := c.CircuitIsEnabled()
	if err != nil {
		return fmt.Errorf("failed to list firewall rules: %w", err)
	}

	if !enabled {
		log.Printf("Circuit %s is already disabled", c.Name)
		return nil
	}

	if err := c.deleteCircuitRulesByComment(); err != nil {
		return fmt.Errorf("failed to remove firewall rules: %w", err)
	}

	if err := c.dropCircuitConntrack(); err != nil {
		return fmt.Errorf("rules removed but failed to drop connections: %w", err)
	}

	log.Printf("Disabled circuit %s successfully", c.Name)
	return nil
}

type CertDir struct {
	Name  string   `json:"name,omitempty"`
	Files []string `json:"files"`
}

type CertListing map[string][]CertDir

type InterfaceStats struct {
	RX uint64 `json:"rx"`
	TX uint64 `json:"tx"`
}
