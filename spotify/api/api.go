package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"example/lastfm-spotify-syncer/config"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

const SPOTIFY_API_URL = "https://api.spotify.com/v1"

func Authorize(authData *config.SpotifyAuthData, code string) error {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	redirectURI := "http://localhost:8000/spotify-auth"

	// Build the request data
	data := url.Values{}
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	// Create a basic authentication header
	authHeader := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))

	// Create an HTTP request
	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		log.Error("Error creating request:", "error", err)
		return err
	}

	req.Header.Set("Authorization", "Basic "+authHeader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Error making request:", "error", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response
	if resp.Status != "200 OK" {
		log.Error("Error: HTTP Status", "status", resp.Status)
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(&authData)
}

func GetAuth() (*config.SpotifyAuthData, error) {
	conf, err := config.LoadConfig(false)
	if err != nil {
		log.Error("Error fetching config", "error", err)
		return nil, err
	}

	// No need to refresh the token if it hasn't expired
	expired := conf.Spotify.ExpiresAt.Before(time.Now())
	if !expired {
		return &conf.Spotify, nil
	}

	err = refreshToken(&conf.Spotify)
	if err != nil {
		log.Error("Error refreshing token")
		return nil, err
	}

	// Set the expiry time
	expiresIn := time.Duration(conf.Spotify.ExpiresIn) * time.Second
	expiresAt := time.Now().Add(expiresIn)
	conf.Spotify.ExpiresAt = expiresAt

	config.WriteConfig(conf)

	return &conf.Spotify, nil
}

func refreshToken(authData *config.SpotifyAuthData) error {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	refreshToken := authData.RefreshToken

	// Define the URL for the token endpoint
	spotifyUrl := "https://accounts.spotify.com/api/token"

	// Create a URL-encoded request body
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	// Create an HTTP request with the request body
	payload := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", spotifyUrl, payload)
	if err != nil {
		log.Error("Error creating request:", "error", err)
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Create and set the "Authorization" header with the Base64-encoded client ID and client secret
	authString := clientID + ":" + clientSecret
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(authString))
	req.Header.Set("Authorization", authHeader)

	// Create an HTTP client and make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Error making request:", "error", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response
	if resp.Status != "200 OK" {
		log.Error("Error: HTTP Status", "status", resp.Status)
		return errors.New("unsuccessful http request")
	}

	log.Info("response", "status", resp.Status)

	return json.NewDecoder(resp.Body).Decode(&authData)
}

func Get[T any](data *T, endpoint string, params map[string]string) error {
	// Get the access token
	authData, err := GetAuth()
	if err != nil {
		log.Error("Error loading config", "error", err)
		return err
	}
	// Create the full endpoint
	completeEndpoint := SPOTIFY_API_URL + endpoint

	// Create a map of query parameters
	queryParams := url.Values{}

	for key, value := range params {
		queryParams.Add(key, value)
	}

	// Encode the query parameters into a URL-encoded string
	query := queryParams.Encode()

	// Build the complete URL with query parameters
	fullURL := fmt.Sprintf("%s?%s", completeEndpoint, query)
	log.Info("full URL", "url", fullURL)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return err
	}

	// Set the User-Agent header
	req.Header.Set("Authorization", "Bearer "+authData.AccessToken)

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Error making the request:", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		log.Warn("failed", "error code", resp.StatusCode)
	}

	// Decode the JSON response into the map
	return json.NewDecoder(resp.Body).Decode(&data)
}
