package main

import (
	"encoding/json"
	"example/lastfm-spotify-syncer/config"
	lastFmApi "example/lastfm-spotify-syncer/lastfm/api"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Load the config file here
	config.LoadConfig(false)

	router := gin.Default()
	router.GET("/ping", getPing)
	router.GET("/authenticate", authenticate)
	router.GET("/lastfm-auth", lastFmCallback)

	router.GET("/toptracks", getLastFmTopTracksMonth)

	router.Run("localhost:8000")
}

type LastFmCallbackData struct {
	Token string `form:"token"`
}

// Handles the authorization callback from lastfm
func lastFmCallback(c *gin.Context) {
	var lastFmCallbackData LastFmCallbackData

	err := c.ShouldBind(&lastFmCallbackData)
	if err != nil {
		log.Error("error reading token from last fm")
	}
	log.Info("received token", "token", lastFmCallbackData.Token)

	data := lastFmApi.AuthData{}
	lastFmApi.Authorize(&data, lastFmCallbackData.Token)

	if err != nil {
		log.Error("error fetching session token", "error", err)
		c.String(http.StatusInternalServerError, "Failed to authorize with LastFM")
		return
	}

	log.Info("Response data", "data", data)
	log.Info("key", "key", data.Session.Key)

	// Now write this to file
	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Error("Error reading config file", "error", err)
		c.String(http.StatusInternalServerError, "Error reading config file")
	}
	conf.LastFM = data.Session.Key
	config.WriteConfig(conf)

	c.String(http.StatusOK, "success")
}

func authenticate(c *gin.Context) {
	lastFmApiKey := os.Getenv("LASTFM_API_KEY")

	// Send user to the web page to authorize
	log.Info("Please follow this url to authenticate", "link", "http://www.last.fm/api/auth/?api_key="+lastFmApiKey)

	c.IndentedJSON(http.StatusOK, "success")
}

func getPing(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, "PONG")
}

func getLastFmTopTracksMonth(c *gin.Context) {
	// var topTracksData interface{}
	var topTracksData lastFmApi.TopTracks

	params := map[string]string{
		"method": "user.getTopTracks",
		"user":   "fuzzycut1",
		"period": "1month",
		"limit":  "2",
	}

	err := lastFmApi.Get(
		&topTracksData,
		params,
	)
	if err != nil {
		log.Error("Unable to fetch from last fm api", "error", err)
		c.String(http.StatusInternalServerError, "Failed to fetch from lastfm")
		return
	}

	log.Info("lastfm data", "data", topTracksData)
}

// Pretty print a struct
func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}
