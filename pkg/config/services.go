// Package config provides configuration loading and parsing functionality.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ServicesConfig is a struct to hold the data loaded by LoadServices
type ServicesConfig struct {
	Lot1Services []string `json:"lot1_services"`
}

// LoadServices is a function which loads the preconfigured services from a json file. It is used to check the validity of a submission payload.
func LoadServices(configPath string) (*ServicesConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ServicesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if len(config.Lot1Services) == 0 {
		return nil, fmt.Errorf("no services defined in config")
	}

	return &config, nil
}
