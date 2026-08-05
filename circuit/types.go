package circuit

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ElementalMP4/loggerino/log"
)

const iptablesTimeout = 10 * time.Second

type CircuitType string

const (
	CircuitRedirect CircuitType = "REDIRECT"
	CircuitDNAT     CircuitType = "DNAT"
	CircuitLocal    CircuitType = "LOCAL"
)

type RoutingCircuit struct {
	ID          int64       `json:"id"`
	Name        string      `json:"name"`
	Type        CircuitType `json:"type"`
	Enabled     bool        `json:"enabled"`
	InInterface *string     `json:"in_interface,omitempty"`

	Protocol string `json:"protocol,omitempty"`
	SrcPort  int    `json:"src_port,omitempty"`

	RedirectPort *int `json:"redirect_port,omitempty"`

	DestIP   *string `json:"dest_ip,omitempty"`
	DestPort *int    `json:"dest_port,omitempty"`
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
	log.Infof("circuits", "Enabling circuit %s", c.Name)

	enabled, err := c.CircuitIsEnabled()
	if err != nil {
		return fmt.Errorf("failed to list firewall rules: %w", err)
	}

	if enabled {
		log.Okf("circuits", "Circuit %s is already enabled", c.Name)
		return nil
	}

	for _, expanded := range c.expandProtocols() { // Loop over TCP & UDP
		for _, rule := range expanded.IPTablesRules() { // Loop over individual iptables commands
			log.Okf("circuits", "Enabling circuit %s protocol %s", expanded.Name, expanded.Protocol)
			ctx, cancel := context.WithTimeout(context.Background(), iptablesTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, "iptables", rule...)

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("error whilst calling iptables: %v", err)
			}
		}
	}
	log.Okf("circuits", "Enabled circuit %s successfully", c.Name)
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
	log.Infof("circuits", "Disabling circuit %s", c.Name)

	enabled, err := c.CircuitIsEnabled()
	if err != nil {
		return fmt.Errorf("failed to list firewall rules: %w", err)
	}

	if !enabled {
		log.Okf("circuits", "Circuit %s is already disabled", c.Name)
		return nil
	}

	if err := c.deleteCircuitRulesByComment(); err != nil {
		return fmt.Errorf("failed to remove firewall rules: %w", err)
	}

	if err := c.dropCircuitConntrack(); err != nil {
		return fmt.Errorf("rules removed but failed to drop connections: %w", err)
	}

	log.Okf("circuits", "Disabled circuit %s successfully", c.Name)
	return nil
}
