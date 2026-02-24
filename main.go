package main

import (
	"Dynamo/api"
	"Dynamo/banner"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/db"
	"log"
)

func main() {
	banner.ShowTheBanner()
	err := db.InitDB()
	if err != nil {
		log.Fatalf("An error occurred whilst starting the db: %v", err.Error())
	}

	err = config.LoadConfig()
	if err != nil {
		log.Fatalf("An error occurred whilst loading the config: %v", err.Error())
	}

	err = circuit.SyncCircuits()
	if err != nil {
		log.Fatalf("An error occurred whilst syncing circuits: %v", err)
	}

	api.StartAPI()
	select {}
}
