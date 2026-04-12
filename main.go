package main

import (
	"Dynamo/api"
	"Dynamo/banner"
	"Dynamo/circuit"

	"github.com/ElementalMP4/loggerino/log"
)

func main() {
	banner.ShowTheBanner()

	err := circuit.SyncCircuits()
	if err != nil {
		log.Fatalf("circuit-sync", "An error occurred whilst syncing circuits: %v", err)
	}

	api.StartAPI()
	select {}
}
