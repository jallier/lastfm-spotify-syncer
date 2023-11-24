package main

import (
	"encoding/json"
	"example/lastfm-spotify-syncer/config"
	lastFmApi "example/lastfm-spotify-syncer/lastfm/api"
	"example/lastfm-spotify-syncer/scheduler"
	spotifyApi "example/lastfm-spotify-syncer/spotify/api"
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

func wrap(syncId string, sync bool) map[string]interface{} {
	log.Debug("Wrap called")
	return map[string]interface{}{
		"syncId": syncId,
		"sync":   sync,
	}
}

func wrapAny(input ...string) map[string]interface{} {
	log.Debug("wrapAny called", "input", input)

	output := make(map[string]interface{})
	for _, v := range input {
		output[v] = v
	}

	log.Debug("wrapAny called", "output", output)
	return output
}

func unflatten(input ...any) map[string]interface{} {
	log.Debug("unflatten", "input", input)

	output := make(map[string]interface{})
	var firstVal any
	for i, v := range input {
		if i%2 == 0 {
			firstVal = v
		} else {
			if value, ok := firstVal.(string); ok {
				output[value] = v
			}
		}
	}

	log.Debug("unflatten", "output", output)
	return output
}

func toTitle(input string) string {
	caser := cases.Title(language.English)
	val := caser.String(input)
	log.Debug("toTitle", "val", val)
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

	// setup the scheduler
	s := scheduler.GetScheduler()
	s.WaitForScheduleAll()

	// Schedule monthly job - hardcoded for now
	_, err = s.Every(1).Month(1).Do(func() {
		log.Info("Running monthly sync job...")
		sync()
		log.Info("Sync job complete")
	})
	if err != nil {
		log.Error("Error scheduling job", "error", err)
	}
	s.StartAsync()

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
		"wrap":      wrap,
		"wrapAny":   wrapAny,
		"title":     toTitle,
		"unflatten": unflatten,
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
	router.GET("/toptracks", handleTopTracks)
	router.GET("/playlists", getSpotifyPlaylists)
	router.GET("/sync", handleSync)

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
		conf.Auth.Spotify.ClientId = credentials.SpotifyClientId
		conf.Auth.Spotify.ClientSecret = credentials.SpotifyClientSecret
		log.Debug("new lastfm api key is", "key", conf.Auth)

		config.WriteConfig(conf)

		c.Redirect(http.StatusFound, "/")
	})

	if conf.Config.Sync.Monthly.Enabled || conf.Config.Sync.Weekly.Enabled {
		log.Info("sync started")
	} else {
		log.Info("sync not enabled; pausing jobs")
		s.PauseJobExecution(true)
	}

	router.Run(":8000")
}

type SetSyncParams struct {
	MaxTracks int `form:"max-tracks"`
}

// Enable or disable the sync for a particular frequency
func setSync(c *gin.Context) {
	frequency := c.Param("frequency")
	var setSyncParams SetSyncParams
	if err := c.ShouldBind(&setSyncParams); err != nil {
		log.Error("error reading input", "error", err)
		c.String(http.StatusInternalServerError, "Error reading MaxTracks parameter")
		return
	}

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
	case "monthly":
		conf.Config.Sync.Monthly.Enabled = !conf.Config.Sync.Monthly.Enabled
		conf.Config.Sync.Monthly.MaxTracks = setSyncParams.MaxTracks
	default:
		log.Warn("Invalid value given", "value", frequency)
		c.String(400, "Invalid value given; must be weekly or monthly")
		return
	}
	config.WriteConfig(conf)

	c.Redirect(http.StatusFound, "/")

	// We need to change scheduler state depending on both settings
	s := scheduler.GetScheduler()
	if conf.Config.Sync.Monthly.Enabled || conf.Config.Sync.Weekly.Enabled {
		s.PauseJobExecution(false)
		log.Info("sync started")
	} else {
		s.PauseJobExecution(true)
		log.Info("sync stopped")
	}
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
	conf.Auth.LastFM.Token = data.Session.Key
	config.WriteConfig(conf)

	c.Redirect(http.StatusFound, "/")
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

func getPing(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, "PONG")
}

func handleTopTracks(c *gin.Context) {
	topTracksData, err := getLastFmTopTracksMonth()
	if err != nil {
		log.Error("Unable to fetch from last fm api", "error", err)
		c.String(http.StatusInternalServerError, "Failed to fetch from lastfm")
		return
	}

	log.Info("lastfm data", "data", topTracksData)
	pretty, err := PrettyStruct(topTracksData)
	if err != nil {
		return
	}
	log.Info("pretty json", "pretty", pretty)
}

func getSpotifyPlaylists(c *gin.Context) {
	// Grab a new access token and update the config with the new values
	authData, err := spotifyApi.GetAuth()
	if err != nil {
		log.Error("Error fetching config", "error", err)
		c.String(http.StatusInternalServerError, "Error getting spotify playlists")
		return
	}
	log.Info("New spotify tokens", "tokens", authData)

	type Playlists struct {
		Total int `json:"total"`
	}

	var playlistsData Playlists
	spotifyApi.Get(&playlistsData, "/me/playlists", nil)

	log.Info("Playlist data", "data", playlistsData)
}

func handleSync(c *gin.Context) {
	err := sync()
	if err != nil {
		log.Error("Error running sync", "error", err)
		c.String(http.StatusInternalServerError, "Error running sync")
		return
	}

	c.HTML(http.StatusOK, "partial/sync-manually", nil)
}

func sync() error {
	topTracksData, err := getLastFmTopTracksMonth()
	if err != nil {
		log.Error("Unable to fetch from last fm api", "error", err)
		return err
	}
	spotifyUserData, err := getSpotifyUser()
	if err != nil {
		log.Error("Unable to fetch from spotify api", "error", err)
		return err
	}

	var trackIds []string

	// Iterate and search for each track
	// TODO: concurrently?
	for _, v := range topTracksData.Toptracks.Track {
		trackName := v.Name
		artistName := v.Artist.Name
		log.Info("track data", "name", trackName, "artist", artistName)

		var searchData spotifyApi.Search
		searchQuery := fmt.Sprintf("artist: \"%s\" track: \"%s\"", artistName, trackName)
		err := spotifyApi.Get(&searchData, "/search", map[string]string{
			"q":     searchQuery,
			"type":  "track",
			"limit": "1",
		})

		if err != nil {
			log.Error("error searching spotify", "error", err)
			continue
		}

		trackIds = append(trackIds, searchData.Tracks.Items[0].ID)
	}
	log.Info("track ids", "ids", trackIds)

	// Create a new playlist
	playlistData, err := createSpotifyPlaylist(spotifyUserData.ID)
	if err != nil {
		log.Error("error creating playlist", "error", err)
		return err
	}
	log.Info("created playlist", "playlist", playlistData)

	// Add the tracks to the new playlist by uri
	_, err = addItemsToSpotifyPlaylist(playlistData.ID, trackIds)
	if err != nil {
		log.Error("error adding items to playlist playlist", "error", err)
		return err // TODO: try delete the blank playlist here
	}

	log.Info("Populated playlist!")
	return nil
}

func addItemsToSpotifyPlaylist(playlistId string, trackIds []string) (spotifyApi.AddPlaylistTracks, error) {
	var playlistSnapshot spotifyApi.AddPlaylistTracks

	formattedTracks := make([]string, len(trackIds))
	for i, v := range trackIds {
		formattedTracks[i] = "spotify:track:" + v
	}

	url := fmt.Sprintf("/playlists/%s/tracks", playlistId)
	body := spotifyApi.AddPlaylistTracksData{
		Uris: formattedTracks,
	}
	err := spotifyApi.Post(&playlistSnapshot, url, &body)

	return playlistSnapshot, err
}

func createSpotifyPlaylist(userId string) (spotifyApi.CreatePlaylist, error) {
	currentTime := time.Now()
	firstDayOfCurrentMonth := time.Date(currentTime.Year(), currentTime.Month(), 1, 0, 0, 0, 0, currentTime.Location())
	lastDayOfPreviousMonth := firstDayOfCurrentMonth.Add(-time.Second)
	previousMonth := lastDayOfPreviousMonth.Month()
	year := lastDayOfPreviousMonth.Year()

	var playlistData spotifyApi.CreatePlaylist

	url := fmt.Sprintf("/users/%s/playlists", userId)
	body := spotifyApi.CreatePlaylistData{
		Name: fmt.Sprintf("LastFM Top Tracks: %s %d", previousMonth, year),
	}
	err := spotifyApi.Post(&playlistData, url, &body)

	return playlistData, err
}

func getSpotifyUser() (spotifyApi.User, error) {
	var userData spotifyApi.User

	err := spotifyApi.Get(&userData, "/me", nil)

	return userData, err
}

func getLastFmTopTracksMonth() (lastFmApi.TopTracks, error) {
	var topTracksData lastFmApi.TopTracks

	params := map[string]string{
		"method": "user.getTopTracks",
		"user":   "fuzzycut1",
		"period": "1month",
		"limit":  "10",
	}

	err := lastFmApi.Get(
		&topTracksData,
		params,
	)

	return topTracksData, err
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
