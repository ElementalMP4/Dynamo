package config

import (
	"Dynamo/types"
	"encoding/json"
	"os"
)

var config types.Config

func LoadConfig() {
	data, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		panic(err)
	}
}

func GetConfig() types.Config {
	return config
}
