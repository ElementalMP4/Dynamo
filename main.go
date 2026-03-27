package main

import (
	"Dynamo/api"
	"Dynamo/banner"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/db"

	"github.com/ElementalMP4/loggerino/log"
)

func main() {
	banner.ShowTheBanner()
	err := db.InitDB()
	if err != nil {
		log.Fatalf("db-start", "An error occurred whilst starting the db: %v", err)
	}

	err = config.LoadConfig()
	if err != nil {
		log.Fatalf("config-load", "An error occurred whilst loading the config: %v", err)
	}

	err = circuit.SyncCircuits()
	if err != nil {
		log.Fatalf("circuit-sync", "An error occurred whilst syncing circuits: %v", err)
	}

	api.StartAPI()
	select {}
}
