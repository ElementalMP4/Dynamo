package main

import (
	"TinyVTM/api"
	"TinyVTM/config"
	"TinyVTM/db"
	"TinyVTM/dns"
	"TinyVTM/log"
)

func main() {
	log.ShowTheBanner()
	config.LoadConfig()
	db.InitDB()
	go api.StartAPI()
	go dns.StartDNS()

	select {}
}
