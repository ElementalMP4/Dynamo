package main

import (
	"Dynamo/api"
	"Dynamo/config"
	"Dynamo/log"
)

func main() {
	log.ShowTheBanner()
	config.LoadConfig()
	go api.StartAPI()

	select {}
}
