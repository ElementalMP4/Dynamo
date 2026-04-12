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

func init() {
	data, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		panic(err)
	}
}

func GetConfig() Config {
	return config
}
