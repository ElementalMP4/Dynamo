package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"

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
	if c.Certificates.RemoteCertURL == "" {
		return true
	}

	clientCertSet := c.Certificates.RemoteCertClientCert != ""
	clientKeySet := c.Certificates.RemoteCertClientKey != ""
	caCertSet := c.Certificates.RemoteCertCACert != ""

	allEmpty := !clientCertSet && !clientKeySet && !caCertSet
	allSet := clientCertSet && clientKeySet && caCertSet

	return allEmpty || allSet
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
