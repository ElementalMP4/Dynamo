package main

import (
	"Dynamo/api"
	"Dynamo/config"
	"Dynamo/db"
	"Dynamo/log"
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

	api.StartAPI()
	select {}
}
