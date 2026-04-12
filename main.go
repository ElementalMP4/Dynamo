package main

import (
	"Dynamo/api"
	"Dynamo/banner"
	"Dynamo/circuit"
	"Dynamo/vpn"

	"github.com/ElementalMP4/loggerino/log"
)

func main() {
	banner.ShowTheBanner()

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
