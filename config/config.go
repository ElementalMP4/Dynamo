package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ElementalMP4/loggerino/log"

	"os"
)

var config Config

type Config struct {
	System        SystemConfig      `json:"system"`
	Certificates  CertificateConfig `json:"certificates"`
	DnsmasqConfig DnsmasqConfig     `json:"dnsmasq"`
}

type DnsmasqConfig struct {
	ListenAddress string   `json:"listenAddress"`
	Port          string   `json:"port"`
	Domain        string   `json:"domain"`
	Interface     []string `json:"interface"`
	Upstream      []string `json:"upstreamDns"`
}

type SystemConfig struct {
	ServerBind    string `json:"serverBind"`
	DnsmasqFile   string `json:"dnsmasqConfigFile"`
	DataDirectory string `json:"dataDirectory"`
	InstanceName  string `json:"instance"`
}

type CertificateConfig struct {
	CertsDir             string `json:"certsDir"`
	RemoteCertURL        string `json:"remoteCertUrl"`
	RemoteCertClientCert string `json:"remoteCertClientCert"`
	RemoteCertClientKey  string `json:"remoteCertClientKey"`
	RemoteCertCACert     string `json:"remoteCertTrust"`
}

func init() {
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("config", "failed to read config: %v", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatalf("config", "failed to unmarshal config: %v", err)
	}
}

func GetConfig() Config {
	return config
}

func (c *Config) CheckConfig() error {
	if c.Certificates.CertsDir == "" {
		return fmt.Errorf("certsDir cannot be empty")
	}

	clientCertSet := c.Certificates.RemoteCertClientCert != ""
	clientKeySet := c.Certificates.RemoteCertClientKey != ""
	caCertSet := c.Certificates.RemoteCertCACert != ""

	allEmpty := !clientCertSet && !clientKeySet && !caCertSet
	allSet := clientCertSet && clientKeySet && caCertSet

	if !allEmpty && !allSet {
		return fmt.Errorf("all mTLS fields must either be set or unset")
	}

	if c.System.ServerBind == "" {
		return fmt.Errorf("serverBind cannot be empty")
	}

	if c.System.DataDirectory == "" {
		return fmt.Errorf("dataDirectory cannot be empty")
	}

	if c.System.DnsmasqFile == "" {
		return fmt.Errorf("dnsmasqConfigFile cannot be empty")
	}

	if c.System.InstanceName == "" {
		return fmt.Errorf("instanceName cannot be empty")
	}

	return nil
}

func (c *Config) RemoteCertModeEnabled() bool {
	return c.Certificates.RemoteCertURL != ""
}

func (c *Config) RemoteCertMtlsEnabled() bool {
	return c.Certificates.RemoteCertCACert != ""
}

func (c *Config) BuildMTLSConfig() (*tls.Config, error) {
	tlsCfg := &tls.Config{}

	cert, err := tls.LoadX509KeyPair(c.Certificates.RemoteCertClientCert, c.Certificates.RemoteCertClientKey)
	if err != nil {
		return nil, fmt.Errorf("loading client cert/key: %w", err)
	}
	tlsCfg.Certificates = []tls.Certificate{cert}

	pem, err := os.ReadFile(c.Certificates.RemoteCertCACert)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("failed to parse CA cert from %s", c.Certificates.RemoteCertCACert)
	}
	tlsCfg.RootCAs = pool

	return tlsCfg, nil
}

func (c Config) CreateDnsmasqConfigFile() string {
	var out strings.Builder

	// Bind Settings
	out.WriteString(fmt.Sprintf("port=%s\n", c.DnsmasqConfig.Port))
	out.WriteString(fmt.Sprintf("listen-address=%s\n", c.DnsmasqConfig.Port))

	for _, iface := range c.DnsmasqConfig.Interface {
		out.WriteString(fmt.Sprintf("interface=%s\n", iface))
	}

	// Domain Configuration
	out.WriteString(fmt.Sprintf("domain=%s\n", c.DnsmasqConfig.Domain))
	out.WriteString(fmt.Sprintf("local=/%s/\n", c.DnsmasqConfig.Domain))

	// Upstream DNS
	for _, dns := range c.DnsmasqConfig.Upstream {
		out.WriteString(fmt.Sprintf("server=%s\n", dns))

	}

	// Hosts Config
	out.WriteString("no-hosts\n")
	out.WriteString(fmt.Sprintf("addn-hosts=%s\n", c.System.DataDirectory+"/hosts"))

	// Config directory
	out.WriteString("conf-dir=/etc/dnsmasq.d\n")

	return out.String()
}
