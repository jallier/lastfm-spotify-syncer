package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"

	logger "github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	// "go.uber.org/zap"
)

// func init() {
// 	err := godotenv.Load()
// 	if err != nil {
// 		// log.Fatal("Error loading .env file")
// 	}

// 	logger := zap.Must(zap.NewProduction())
// 	if os.Getenv("APP_ENV") == "development" {
// 		logger = zap.Must(zap.NewDevelopment())
// 	}
// 	zap.ReplaceGlobals(logger)
// }

// Fix this later
// var logger = zap.Must(zap.NewDevelopment()).Sugar()

func main() {
	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Error loading .env file")
	}

	router := gin.Default()
	router.GET("/ping", getPing)
	router.GET("/json", writeJSON)
	router.GET("/authenticate", authenticate)
	router.GET("/lastfm-auth", lastFmCallback)

	router.Run("localhost:8000")
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

	logger.Info("sorted map", "values", output)

	return output
}

func encodeLastFmCall(sortedParams string) string {
	// Create an MD5 hash of the input string
	hash := md5.Sum([]byte(sortedParams))

	// Convert the hash to a hexadecimal representation
	hashHex := hex.EncodeToString(hash[:])

	return hashHex
}

func getLastFmApi(params map[string]string, target interface{}) error {
	const LASTFM_API_URL = "https://ws.audioscrobbler.com/2.0"
	lastFmApiKey := os.Getenv("LASTFM_API_KEY")

	// Create a map of query parameters
	queryParams := url.Values{}
	queryParams.Add("api_key", lastFmApiKey)

	for key, value := range params {
		queryParams.Add(key, value)
	}

	sortedParamString := getSortedMapKV(queryParams)
	fullSigString := sortedParamString + os.Getenv("LASTFM_SHARED_SECRET")
	hashedSignature := encodeLastFmCall(fullSigString)
	logger.Info("hashed signature", hashedSignature)

	// Add format afterwards for some unknown reason...
	queryParams.Add("format", "json")
	queryParams.Add("api_sig", hashedSignature)

	// Encode the query parameters into a URL-encoded string
	query := queryParams.Encode()

	// Build the complete URL with query parameters
	fullURL := fmt.Sprintf("%s?%s", LASTFM_API_URL, query)
	logger.Info("full URL", fullURL)

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
		logger.Error("Error making the request:", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		logger.Warn("failed", "error code", resp.StatusCode)
	}

	// Read the response body
	// body, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	return nil, err
	// }

	// Create a map to hold the JSON data

	// Decode the JSON response into the map
	return json.NewDecoder(resp.Body).Decode(&target)
}

type LastFmCallbackData struct {
	Token string `form:"token"`
}

// Handles the authorization callback from lastfm
func lastFmCallback(c *gin.Context) {
	var lastFmCallbackData LastFmCallbackData

	err := c.ShouldBind(&lastFmCallbackData)
	if err != nil {
		logger.Error("error reading token from last fm")
	}
	logger.Info("received token ", lastFmCallbackData.Token)

	type AuthDataSession struct {
		Key        string `json:"key"`
		Name       string `json:"name"`
		Subscriber int    `json:"subscriber"`
	}
	type AuthData struct {
		Session AuthDataSession `json:"session"`
	}

	data := AuthData{}

	err = getLastFmApi(map[string]string{
		"method": "auth.getSession",
		"token":  lastFmCallbackData.Token,
	}, &data)
	if err != nil {
		logger.Error("error fetching session token", "error", err)
	}

	logger.Info("Response data", "data", data)
	logger.Info("key", "key", data.Session.Key)

	// Now write this to file

	c.String(http.StatusOK, "success")
}

func authenticate(c *gin.Context) {
	lastFmApiKey := os.Getenv("LASTFM_API_KEY")

	// Send user to the web page to authorize
	logger.Info("Please follow this url to authenticate", "link", "http://www.last.fm/api/auth/?api_key="+lastFmApiKey)

	c.IndentedJSON(http.StatusOK, "success")
}

func getPing(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, "PONG")
}

type TokenData struct {
	LastFM string `json:"last_fm"`
}

func writeJSON(c *gin.Context) {
	data := TokenData{"Test Last FM"}
	fmt.Println("data is", data.LastFM)

	// Create or open a file for writing.
	file, err := os.Create("tokens.json")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close() // Ensure the file is closed when we're done.

	// Create a JSON encoder and encode the struct into JSON format.
	encoder := json.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}

	fmt.Println("JSON data written to person.json")

	c.Status(http.StatusOK)
}
