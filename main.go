package main

import (
	"Dynamo/api"
	"Dynamo/circuit"
	"Dynamo/config"
	"Dynamo/db"
	"Dynamo/log"
	"fmt"
)

func main() {
	log.ShowTheBanner()
	err := db.InitDB()
	if err != nil {
		log.LogFatal(err.Error())
	}

	err = config.LoadConfig()
	if err != nil {
		log.LogFatal(err.Error())
	}

	err = circuit.SyncCircuits()
	if err != nil {
		log.LogWarn(fmt.Sprintf("An error occurred whilst syncing circuits: %v", err))
	}

	api.StartAPI()
	select {}
}
