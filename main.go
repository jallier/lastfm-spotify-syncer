package main

import (
	"encoding/json"
	"example/lastfm-spotify-syncer/config"
	lastFmApi "example/lastfm-spotify-syncer/lastfm/api"
	spotifyApi "example/lastfm-spotify-syncer/spotify/api"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

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

	// Endpoint to send links user needs to follow to auth with both services
	router.GET("/authenticate", authenticate)

	// Endpoints to handle oauth callbacks
	router.GET("/lastfm-auth", lastFmCallback)
	router.GET("/spotify-auth", spotifyCallback)

	// Data endpoints
	router.GET("/toptracks", getLastFmTopTracksMonth)
	router.GET("/playlists", getSpotifyPlaylists)

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

type SpotifyCallbackData struct {
	Code  string `form:"code"`
	State string `form:"state"`
}

func spotifyCallback(c *gin.Context) {
	var spotifyCallbackData SpotifyCallbackData

	err := c.ShouldBind(&spotifyCallbackData)
	if err != nil {
		log.Error("error reading code from spotify")
		c.String(http.StatusInternalServerError, "Unable to read code from spotify")
		return
	}
	log.Info("Spotify code", "code", spotifyCallbackData.Code)

	var authData spotifyApi.AuthData

	err = spotifyApi.Authorize(&authData, spotifyCallbackData.Code)
	if err != nil {
		log.Error("Error parsing json", "error", err)
		c.String(http.StatusInternalServerError, "Error parsing spotify json")
		return
	}

	// Now write the tokens to file
	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Error("Error reading config file", "error", err)
		c.String(http.StatusInternalServerError, "Error reading config file")
		return
	}
	conf.Spotify = authData
	config.WriteConfig(conf)

	c.String(http.StatusOK, "success")
}

func authenticate(c *gin.Context) {
	lastFmApiKey := os.Getenv("LASTFM_API_KEY")
	spotifyApiKey := os.Getenv("SPOTIFY_CLIENT_ID")

	// TODO: these should just redirect the user to the services. But worry about that when there is a ui

	// Last fm - ezpz
	log.Info("Please follow this url to authenticate lastFM", "link", "http://www.last.fm/api/auth/?api_key="+lastFmApiKey)

	// Spotify - gotta encode some stuff
	scopes := "playlist-read-private playlist-modify-private"
	redirectUrl := "http://localhost:8000/spotify-auth"
	queryParams := url.Values{}
	queryParams.Add("response_type", "code")
	queryParams.Add("client_id", spotifyApiKey)
	queryParams.Add("scope", scopes)
	queryParams.Add("redirect_uri", redirectUrl)
	// TODO: put this into a cookie and compare the value in the next endpoint to make sure they haven't changed
	queryParams.Add("state", randomString(16))

	queryString := queryParams.Encode()
	spotifyURL := "https://accounts.spotify.com/authorize"
	fullSpotifyURL := fmt.Sprintf("%s?%s", spotifyURL, queryString)
	log.Info("Please follow this url to authenticate spotify", "link", fullSpotifyURL)

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

func getSpotifyPlaylists(c *gin.Context) {
	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Error("Error fetching config", "error", err)
		c.String(http.StatusInternalServerError, "Error getting spotify playlists")
		return
	}

	// Grab a new access token and update the config with the new values
	err = spotifyApi.GetAuth(&conf.Spotify)
	if err != nil {
		log.Error("Error fetching config", "error", err)
		c.String(http.StatusInternalServerError, "Error getting spotify playlists")
		return
	}
	log.Info("New spotify tokens", "tokens", conf.Spotify)
	config.WriteConfig(conf)

	type Playlists struct {
		Total int `json:"total"`
	}

	var playlistsData Playlists
	// spotifyApi.Get(&playlistsData, "/me/playlists", conf.Spotify.AccessToken, map[string]string{})
	spotifyApi.Get(&playlistsData, "/me/playlists", conf.Spotify.AccessToken, nil)

	log.Info("Playlist data", "data", playlistsData)
}

// Pretty print a struct
func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// Generate a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	seededRand := rand.New(
		rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}
