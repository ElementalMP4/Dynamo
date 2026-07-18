package config

import (
	"encoding/json"

	"github.com/ElementalMP4/loggerino/log"

	"os"
)

type Config struct {
	System       SystemConfig      `json:"system"`
	Certificates CertificateConfig `json:"certificates"`
}

type SystemConfig struct {
	ServerAddress string `json:"serverBind"`
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

var config Config

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

func (c *Config) CheckCertMode() bool {
	// If certs directory is set, require that remote URL and all remote TLS values are empty
	if c.Certificates.CertsDir != "" {
		return c.Certificates.RemoteCertURL == "" && c.Certificates.RemoteCertClientCert == "" && c.Certificates.RemoteCertClientKey == "" && c.Certificates.RemoteCertCACert == ""
	}

	// If certs directory is not set, require RemoteCertURL and ensure TLS values
	// are either all empty or all non-empty
	if c.Certificates.RemoteCertURL == "" {
		return false
	}

	clientCertSet := c.Certificates.RemoteCertClientCert != ""
	clientKeySet := c.Certificates.RemoteCertClientKey != ""
	caCertSet := c.Certificates.RemoteCertCACert != ""

	allEmpty := !clientCertSet && !clientKeySet && !caCertSet
	allSet := clientCertSet && clientKeySet && caCertSet

	return allEmpty || allSet
}

func (c *Config) RemoteCertModeEnabled() bool {
	return c.Certificates.CertsDir == ""
}

func (c *Config) RemoteCertMtlsEnabled() bool {
	return c.Certificates.RemoteCertCACert != ""
}
