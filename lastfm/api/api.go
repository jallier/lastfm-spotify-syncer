package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"

	"github.com/charmbracelet/log"
)

const LASTFM_API_URL = "https://ws.audioscrobbler.com/2.0"

type AuthData struct {
	Session struct {
		Key        string `json:"key"`
		Name       string `json:"name"`
		Subscriber int    `json:"subscriber"`
	} `json:"session"`
}

// Hit the lastfm api to authorize the user
// This will handle the hashing signature requirement
func Authorize(authData *AuthData, token string) error {
	lastFmApiKey := os.Getenv("LASTFM_API_KEY")

	// Create a map of query parameters
	queryParams := url.Values{}
	queryParams.Add("api_key", lastFmApiKey)
	queryParams.Add("token", token)
	queryParams.Add("method", "auth.getSession")

	sortedParamString := getSortedMapKV(queryParams)
	fullSigString := sortedParamString + os.Getenv("LASTFM_SHARED_SECRET")
	hashedSignature := encodeLastFmCall(fullSigString)
	log.Info("hashed signature", "sig", hashedSignature)

	// Add format afterwards for some unknown reason...
	queryParams.Add("format", "json")
	queryParams.Add("api_sig", hashedSignature)

	// Encode the query parameters into a URL-encoded string
	query := queryParams.Encode()

	// Build the complete URL with query parameters
	fullURL := fmt.Sprintf("%s?%s", LASTFM_API_URL, query)
	log.Info("full URL", "url", fullURL)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return err
	}

	// Set the User-Agent header
	req.Header.Set("User-Agent", "lastfm-spotify-syncer")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Error making the request:", "error", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		log.Warn("failed", "error code", resp.StatusCode)
	}

	// Decode the JSON response into the map
	return json.NewDecoder(resp.Body).Decode(&authData)
}

func Get[T any](data *T, params map[string]string) error {
	lastFmApiKey := os.Getenv("LASTFM_API_KEY")

	// Create a map of query parameters
	queryParams := url.Values{}
	queryParams.Add("api_key", lastFmApiKey)

	for key, value := range params {
		queryParams.Add(key, value)
	}

	// Add format afterwards for some unknown reason...
	queryParams.Add("format", "json")
	// queryParams.Add("api_sig", hashedSignature)

	// Encode the query parameters into a URL-encoded string
	query := queryParams.Encode()

	// Build the complete URL with query parameters
	fullURL := fmt.Sprintf("%s?%s", LASTFM_API_URL, query)
	log.Info("full URL", "url", fullURL)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return err
	}

	// Set the User-Agent header
	req.Header.Set("User-Agent", "lastfm-spotify-syncer")

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

func getSortedMapKV(data url.Values) string {
	// Extract and sort the keys
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Iterate through the sorted keys
	var output string
	for _, key := range keys {
		value := data[key]
		for _, val := range value {
			output += fmt.Sprintf("%s%s", key, val)
		}
	}

	log.Info("sorted map", "values", output)

	return output
}

func encodeLastFmCall(sortedParams string) string {
	// Create an MD5 hash of the input string
	hash := md5.Sum([]byte(sortedParams))

	// Convert the hash to a hexadecimal representation
	hashHex := hex.EncodeToString(hash[:])

	return hashHex
}
