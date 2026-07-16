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
	if c.Certificates.CertsDir == "" {
		tlsValid := !(c.Certificates.RemoteCertClientCert != "" || c.Certificates.RemoteCertClientKey != "" || c.Certificates.RemoteCertCACert != "")
		return tlsValid && c.Certificates.RemoteCertURL != ""
	}
}

func (c *Config) RemoteCertModeEnabled() bool {
	return c.Certificates.CertsDir == ""
}
