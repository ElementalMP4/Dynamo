package main

import (
	"Dynamo/api"
	"Dynamo/banner"
	"Dynamo/certificate"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/dns"
	"Dynamo/vpn"
	"os"

	"github.com/ElementalMP4/loggerino/log"
)

func main() {
	banner.ShowTheBanner()

	config := config.GetConfig()
	if _, err := os.Stat(config.System.DataDirectory); os.IsNotExist(err) {
		err := os.MkdirAll(config.System.DataDirectory, 0755)
		if err != nil {
			log.Fatalf("config", "Failed to create data directory: %v", err)
		}
	}

	err := config.CheckConfig()
	if err != nil {
		log.Fatalf("config", "Invalid config: %v", err)
	}

	if config.RemoteCertModeEnabled() {
		log.Infof("certificates", "Using certificates from %s", config.Certificates.RemoteCertURL)
	} else {
		if err := certificate.EnsureSslDirs(); err != nil {
			log.Fatalf("certificates", "Failed to check SSL dirs: %v", err)
		}
	}

	if err := circuit.SyncCircuits(); err != nil {
		log.Fatalf("circuit-sync", "An error occurred whilst syncing circuits: %v", err)
	}

	if err := vpn.SyncWireguard(); err != nil {
		log.Fatalf("vpn-sync", "An error occurred whilst syncing VPN state: %v", err)
	}

	if err := dns.SyncDNS(); err != nil {
		log.Fatalf("dns-sync", "An error occurred whilst syncing DNS state: %v", err)
	}

	go api.StartAPI()
	go dns.StartDNS()
	select {}
}
