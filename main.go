package main

import (
	"Dynamo/api"
	"Dynamo/banner"
	"Dynamo/circuit"
	"Dynamo/config"
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

	err := circuit.SyncCircuits()
	if err != nil {
		log.Fatalf("circuit-sync", "An error occurred whilst syncing circuits: %v", err)
	}

	err = vpn.SyncWireguard()
	if err != nil {
		log.Fatalf("vpn-sync", "An error occurred whilst syncing VPN state: %v", err)
	}

	api.StartAPI()
	select {}
}
