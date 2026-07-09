package config

import (
	"encoding/json"

	"github.com/ElementalMP4/loggerino/log"

	"os"
)

type Config struct {
	ServerAddress string `json:"serverBind"`
	CertsDir      string `json:"certsDir"`
	DnsmasqFile   string `json:"dnsmasqConfigFile"`
	DataDirectory string `json:"dataDirectory"`
	InstanceName  string `json:"instance"`
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
