package main

import (
	"Dynamo/api"
	"Dynamo/config"
	"Dynamo/db"
	"Dynamo/dns"
	"Dynamo/log"
)

func main() {
	log.ShowTheBanner()
	config.LoadConfig()
	db.InitDB()
	go api.StartAPI()
	go dns.StartDNS()

	select {}
}
