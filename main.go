package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	router.GET("/ping", getPing)
	router.GET("/json", writeJSON)
	router.GET("/authenticate", authenticate)

	router.Run("localhost:8000")
}

func authenticate(c *gin.Context) {
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
