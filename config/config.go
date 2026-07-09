package config

import (
	"crypto/tls"
	"encoding/json"
	"net/http"

	"github.com/ElementalMP4/loggerino/log"

	"os"
)

type Config struct {
	ServerAddress string                      `json:"serverBind"`
	CertsDir      string                      `json:"certsDir"`
	DnsmasqFile   string                      `json:"dnsmasqConfigFile"`
	DataDirectory string                      `json:"dataDirectory"`
	InstanceName  string                      `json:"instance"`
	CertProvider  *UpstreamCertficateProvider `json:"upstreamCertProvider"`
}

type UpstreamCertficateProvider struct {
	DynamoEndpoint  string `json:"endpoint"`
	CertificatePath string `json:"cert"`
	KeyPath         string `json:"key"`
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

func (u *UpstreamCertficateProvider) CreateHttpsClient() (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(u.CertificatePath, u.KeyPath)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}
	client := &http.Client{Transport: tr}
	return client, nil
}
