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
	if _, err := os.Stat(config.DataDirectory); os.IsNotExist(err) {
		err := os.MkdirAll(config.DataDirectory, 0755)
		if err != nil {
			log.Fatalf("config", "Failed to create data directory: %v", err)
		}
	}

	if config.RemoteCertMode {
		log.Warnf("certificates", "Remote certificate mode is enabled — certificate operations will be proxied to %s. No certificate files will be written to disk on this instance.", config.RemoteCertURL)
	} else {
		if err := certificate.EnsureSslDirs(); err != nil {
			log.Fatalf("certificates", "Failed to check SSL dirs: %v", err)
		}
	}

	if err := dns.EnsureWritable(); err != nil {
		log.Fatalf("dns", "DNS config file is not writable: %v", err)
	}

	if err := circuit.SyncCircuits(); err != nil {
		log.Fatalf("circuit-sync", "An error occurred whilst syncing circuits: %v", err)
	}

	if err := vpn.SyncWireguard(); err != nil {
		log.Fatalf("vpn-sync", "An error occurred whilst syncing VPN state: %v", err)
	}

	if err := dns.SyncDns(); err != nil {
		log.Fatalf("dns-sync", "An error occurred whilst syncing DNS state: %v", err)
	}

	api.StartAPI()
	select {}
}
