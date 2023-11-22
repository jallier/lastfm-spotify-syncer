package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

type LastFMAuthData struct {
	ApiKey       string `json:"api_key"`
	SharedSecret string `json:"shared_secret"`
	Token        string `json:"token"`
}

type SpotifyAuthData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	ClientId     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
}

type Config struct {
	Auth struct {
		LastFM  LastFMAuthData  `json:"last_fm"`
		Spotify SpotifyAuthData `json:"spotify"`
	} `json:"auth"`
	Config struct {
		Sync struct {
			Weekly  bool `json:"weekly"`
			Monthly bool `json:"monthly"`
		} `json:"sync"`
	} `json:"config"`
}

const FILENAME = "conf/config.json"

var appEnv string = "NIL"

func IsDev() bool {
	if appEnv == "NIL" {
		err := godotenv.Load()
		appEnv = strings.ToLower(os.Getenv("APP_ENV"))
		if err != nil {
			log.Info("Error loading .env file. Either one not provided or running in prod mode")
		}
	}
	return appEnv == "dev" || appEnv == "development"
}

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

	err := os.MkdirAll(filepath.Dir(filename), 0775)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	configFile, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0660)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return &config, nil
}

func WriteConfig(data *Config) error {
	// Create or open a file for writing.
	file, err := os.Create(FILENAME)
	if err != nil {
		log.Error("Error creating file", "error", err)
		return err
	}
	defer file.Close() // Ensure the file is closed when we're done.

	// Create a JSON encoder and encode the struct into JSON format.
	encoder := json.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		log.Error("Error encoding JSON:", "error", err)
		return err
	}

	log.Info("JSON data written to " + FILENAME)
	return nil
}
