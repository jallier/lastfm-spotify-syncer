package config

import (
	"encoding/json"
	"os"

	"github.com/charmbracelet/log"
)

type Config struct {
	LastFM  string `json:"last_fm"`
	Spotify string `json:"spotify"`
}

const FILENAME = "tokens.json"

var cachedData *Config

// Load the config file.
// Config will be loaded from cache unless force is true
func LoadConfig(force bool) (*Config, error) {
	if cachedData != nil && !force {
		return cachedData, nil
	}

	// Read the JSON file and unmarshal it into a struct
	data, err := readConfigFile(FILENAME)
	if err != nil {
		return nil, err
	}

	// Cache the data
	cachedData = data

	return data, nil
}

func readConfigFile(filename string) (*Config, error) {
	var config Config

	configFile, err := os.Open(filename)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return &config, nil
}

func WriteConfig(data *Config) {
	// Create or open a file for writing.
	file, err := os.Create(FILENAME)
	if err != nil {
		log.Error("Error creating file", "error", err)
		return
	}
	defer file.Close() // Ensure the file is closed when we're done.

	// Create a JSON encoder and encode the struct into JSON format.
	encoder := json.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		log.Error("Error encoding JSON:", "error", err)
		return
	}

	log.Info("JSON data written to " + FILENAME)
}
