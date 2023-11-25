package main

import (
	"example/lastfm-spotify-syncer/config"
	lastFmApi "example/lastfm-spotify-syncer/lastfm/api"
	"example/lastfm-spotify-syncer/scheduler"
	spotifyApi "example/lastfm-spotify-syncer/spotify/api"
	"example/lastfm-spotify-syncer/sync"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Template function - used to convert a string to Title Case
func toTitle(input string) string {
	caser := cases.Title(language.English)
	val := caser.String(input)
	return val
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Info("Error loading .env file. Either one not provided or running in prod mode")
	}

	if config.IsDev() {
		log.SetLevel(log.DebugLevel)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Load the config file here
	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Fatal("Cannot load config", "error", err)
	}

	// Setup
	router := gin.Default()
	if config.IsDev() {
		router.ForwardedByClientIP = true
		router.SetTrustedProxies([]string{"127.0.0.1"})
	}
	router.SetFuncMap(template.FuncMap{
		"title": toTitle,
	})
	router.LoadHTMLGlob("templates/**/*.tmpl")
	router.Static("/static", "./static")

	router.GET("/ping", getPing)

	// HTML routes
	router.GET("/", func(c *gin.Context) {
		conf, err := config.LoadConfig(false)
		if err != nil {
			log.Error("Error reading config", "error", err)
			c.String(http.StatusInternalServerError, "Error reading config file")
			return
		}
		signedIn := false
		// TODO: Need the client ids and secrets to be checked too?
		if conf.Auth.LastFM.Token != "" && conf.Auth.Spotify.RefreshToken != "" {
			signedIn = true
		}

		c.HTML(http.StatusOK, "index", gin.H{
			// TODO: Better if these are a typesafe struct, but handle that later
			"credentials": []map[string]string{
				{
					"id":    "lastfm-api-key",
					"value": conf.Auth.LastFM.ApiKey,
					"title": "LastFM Api Key",
				},
				{
					"id":    "lastfm-shared-secret",
					"value": conf.Auth.LastFM.SharedSecret,
					"title": "LastFM Shared Secret",
				},
				{
					"id":    "lastfm-username",
					"value": conf.Auth.LastFM.Username,
					"title": "LastFM username",
				},
				{
					"id":    "spotify-client-id",
					"value": conf.Auth.Spotify.ClientId,
					"title": "Spotify Client Id",
				},
				{
					"id":    "spotify-client-secret",
					"value": conf.Auth.Spotify.ClientSecret,
					"title": "Spotify Client Secret",
				},
			},
			"signedIn": signedIn,
			"sync": []map[string]any{
				{
					"syncId":    "weekly",
					"sync":      conf.Config.Sync.Weekly.Enabled,
					"maxTracks": conf.Config.Sync.Weekly.MaxTracks,
				},
				{
					"syncId":    "monthly",
					"sync":      conf.Config.Sync.Monthly.Enabled,
					"maxTracks": conf.Config.Sync.Monthly.MaxTracks,
				},
			},
		})
	})

	// Endpoint to send links user needs to follow to auth with both services
	router.GET("/authenticate-last-fm", authenticateLastFM)
	router.GET("/authenticate-spotify", authenticateSpotify)

	// Endpoints to handle oauth callbacks
	router.GET("/lastfm-auth", lastFmCallback)
	router.GET("/spotify-auth", spotifyCallback)

	// Data endpoints
	router.GET("/sync/:frequency", handleSync)

	// admin endpoints
	router.POST("/admin/set-sync/:frequency", setSync)
	router.POST("/admin/credentials", func(c *gin.Context) {
		conf, err := config.LoadConfig(true)
		if err != nil {
			log.Error("Error reading config", "error", err)
			c.String(http.StatusInternalServerError, "Error reading config file")
			return
		}

		type Credentials struct {
			LastFMApiKey        string `form:"lastfm-api-key"`
			LastFmSharedSecret  string `form:"lastfm-shared-secret"`
			LastFmUsername      string `form:"lastfm-username"`
			SpotifyClientId     string `form:"spotify-client-id"`
			SpotifyClientSecret string `form:"spotify-client-secret"`
		}
		var credentials Credentials
		err = c.Bind(&credentials)
		if err != nil {
			log.Error("Error reading form data", "error", err)
			c.String(http.StatusInternalServerError, "Error reading form data")
			return
		}

		conf.Auth.LastFM.ApiKey = credentials.LastFMApiKey
		conf.Auth.LastFM.SharedSecret = credentials.LastFmSharedSecret
		conf.Auth.LastFM.Username = credentials.LastFmUsername
		conf.Auth.Spotify.ClientId = credentials.SpotifyClientId
		conf.Auth.Spotify.ClientSecret = credentials.SpotifyClientSecret
		log.Debug("new lastfm api key is", "key", conf.Auth)

		config.WriteConfig(conf)

		c.Redirect(http.StatusFound, "/")
	})

	jobTags := [2]string{}
	if conf.Config.Sync.Weekly.Enabled {
		jobTags[0] = "weekly"
	}
	if conf.Config.Sync.Monthly.Enabled {
		jobTags[1] = "monthly"
	}

	err = scheduler.SetupSchedule(jobTags[:])
	if err != nil {
		log.Error("Error setting up scheduler, jobs will not fire", "err", err)
	}

	router.Run(":8000")
}

// Enable or disable the sync for a particular frequency
func setSync(c *gin.Context) {
	type SetSyncParams struct {
		MaxTracks int `form:"max-tracks"`
	}
	var setSyncParams SetSyncParams
	if err := c.ShouldBind(&setSyncParams); err != nil {
		log.Error("error reading input", "error", err)
		c.String(http.StatusInternalServerError, "Error reading MaxTracks parameter")
		return
	}
	frequency := c.Param("frequency")

	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Error("error loading config", "error", err)
		c.String(http.StatusInternalServerError, "Error loading config file")
		return
	}

	validatedFrequency := strings.ToLower(frequency)
	switch validatedFrequency {
	case "weekly":
		conf.Config.Sync.Weekly.Enabled = !conf.Config.Sync.Weekly.Enabled
		conf.Config.Sync.Weekly.MaxTracks = setSyncParams.MaxTracks
		if conf.Config.Sync.Weekly.Enabled {
			scheduler.StartJob("weekly")
		} else {
			scheduler.StopJob("weekly")
		}
	case "monthly":
		conf.Config.Sync.Monthly.Enabled = !conf.Config.Sync.Monthly.Enabled
		conf.Config.Sync.Monthly.MaxTracks = setSyncParams.MaxTracks
		if conf.Config.Sync.Monthly.Enabled {
			scheduler.StartJob("monthly")
		} else {
			scheduler.StopJob("monthly")
		}
	default:
		log.Warn("Invalid value given", "value", frequency)
		c.String(400, "Invalid value given; must be weekly or monthly")
		return
	}
	config.WriteConfig(conf)

	c.Redirect(http.StatusFound, "/")
}

// Handles the authorization callback from lastfm
func lastFmCallback(c *gin.Context) {
	type LastFmCallbackData struct {
		Token string `form:"token"`
	}
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
	conf.Auth.LastFM.Token = data.Session.Key
	config.WriteConfig(conf)

	c.Redirect(http.StatusFound, "/")
}

// Handles the auth callback for spotify
func spotifyCallback(c *gin.Context) {
	type SpotifyCallbackData struct {
		Code  string `form:"code"`
		State string `form:"state"`
	}
	var spotifyCallbackData SpotifyCallbackData

	err := c.ShouldBind(&spotifyCallbackData)
	if err != nil {
		log.Error("error reading code from spotify")
		c.String(http.StatusInternalServerError, "Unable to read code from spotify")
		return
	}
	log.Info("Spotify code", "code", spotifyCallbackData.Code)

	var authData config.SpotifyAuthData

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
	conf.Auth.Spotify = authData
	config.WriteConfig(conf)

	c.Redirect(http.StatusFound, "/")
}

// Generates a link to authenticate with lastfm
func authenticateLastFM(c *gin.Context) {
	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Error("Error reading config file", "error", err)
		c.String(http.StatusInternalServerError, "Error reading config file")
		return
	}
	lastFmApiKey := conf.Auth.LastFM.ApiKey
	link := "http://www.last.fm/api/auth/?api_key=" + lastFmApiKey
	log.Info("Please follow this url to authenticate lastFM", "link", link)
	c.Redirect(http.StatusFound, link)
}

// generates a link to authenticate with spotify
func authenticateSpotify(c *gin.Context) {
	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Error("Error reading config file", "error", err)
		c.String(http.StatusInternalServerError, "Error reading config file")
		return
	}
	spotifyClientId := conf.Auth.Spotify.ClientId
	scopes := "playlist-read-private playlist-modify-private"
	redirectUrl := "http://localhost:8000/spotify-auth"
	queryParams := url.Values{}
	queryParams.Add("response_type", "code")
	queryParams.Add("client_id", spotifyClientId)
	queryParams.Add("scope", scopes)
	queryParams.Add("redirect_uri", redirectUrl)
	// TODO: put this into a cookie and compare the value in the next endpoint to make sure they haven't changed
	queryParams.Add("state", randomString(16))

	queryString := queryParams.Encode()
	spotifyURL := "https://accounts.spotify.com/authorize"
	fullSpotifyURL := fmt.Sprintf("%s?%s", spotifyURL, queryString)
	log.Info("Please follow this url to authenticate spotify", "link", fullSpotifyURL)
	c.Redirect(http.StatusFound, fullSpotifyURL)
}

// Simple ping function
func getPing(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, "PONG")
}

// Handle manually syncing a given period
func handleSync(c *gin.Context) {
	frequency := c.Param("frequency")
	err := sync.Sync(frequency)
	if err != nil {
		log.Error("Error running sync", "error", err)
		c.String(http.StatusInternalServerError, "Error running sync")
		return
	}

	c.HTML(http.StatusOK, "partial/sync-manually", nil)
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
