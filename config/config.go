package config

import (
	"Dynamo/types"
	"encoding/json"
	"os"
)

var config types.Config

func LoadConfig() error {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	return nil
}

func GetConfig() types.Config {
	return config
}
