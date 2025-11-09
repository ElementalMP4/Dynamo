package types

import (
	"fmt"
	"net"
	"strings"
)

type DnsRecord struct {
	ID    int64  `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Type  string `json:"type"`            // "A", "CNAME", "TXT", "SRV"
	TTL   uint32 `json:"ttl,omitempty"`   // time to live in seconds
	Class string `json:"class,omitempty"` // usually "IN"

	// A record (IPv4)
	IPv4 net.IP `json:"ipv4,omitempty"`

	// CNAME record
	CNAME string `json:"cname,omitempty"` // canonical name target

	// TXT record
	TXT []string `json:"txt,omitempty"` // one or more text strings

	// SRV record
	Priority uint16 `json:"priority,omitempty"`
	Weight   uint16 `json:"weight,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	Target   string `json:"target,omitempty"` // target host for SRV
}

// ToSrvResponse converts the DnsRecord to a string representation for SRV records.
func (r *DnsRecord) ToSrvResponse() string {
	return fmt.Sprintf("%d %d %d %s", r.Priority, r.Weight, r.Port, r.Target)
}

// ToTxtResponse converts the DnsRecord to a string representation for TXT records.
func (r *DnsRecord) ToTxtResponse() string {
	return strings.Join(r.TXT, ", ")
}

func (r *DnsRecord) NameTrailingDot() string {
	return r.Name + "."
}

type Config struct {
	UpstreamDns   []string `json:"upstreamDns"`
	DnsAddress    string   `json:"dnsBind"`
	ServerAddress string   `json:"serverBind"`
	CertsDir      string   `json:"certsDir"`
	// NGINX paths (optional - defaults used if empty)
	NginxSitesAvailable string `json:"nginxSitesAvailable,omitempty"`
	NginxSitesEnabled   string `json:"nginxSitesEnabled,omitempty"`
	NginxServiceName    string `json:"nginxServiceName,omitempty"`
}

// GetNginxSitesAvailable returns the sites-available path (default /etc/nginx/sites-available)
func (c Config) GetNginxSitesAvailable() string {
	if c.NginxSitesAvailable != "" {
		return c.NginxSitesAvailable
	}
	return "/etc/nginx/sites-available"
}

// GetNginxSitesEnabled returns the sites-enabled path (default /etc/nginx/sites-enabled)
func (c Config) GetNginxSitesEnabled() string {
	if c.NginxSitesEnabled != "" {
		return c.NginxSitesEnabled
	}
	return "/etc/nginx/sites-enabled"
}

// GetNginxServiceName returns the systemd service name (default nginx)
func (c Config) GetNginxServiceName() string {
	if c.NginxServiceName != "" {
		return c.NginxServiceName
	}
	return "nginx"
}
