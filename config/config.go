package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	ServerAddress string `json:"serverBind"`
	CertsDir      string `json:"certsDir"`
	DnsmasqFile   string `json:"dnsmasqConfigFile"`
	DataDirectory string `json:"dataDirectory"`
}

var config Config

func LoadConfig() error {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	return nil
}

func GetConfig() Config {
	return config
}
