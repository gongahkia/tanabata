// ----- required imports -----

package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"github.com/gin-gonic/gin"
)

// ----- type definitions -----

type Quote struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

// ----- initialization code -----

var quotes []Quote

// ----- helper functions -----

func main() {

	loadQuotes()
	
	r := gin.Default()
	
	r.GET("/quotes", func(c *gin.Context) {
		c.JSON(http.StatusOK, quotes)
	})
	
	r.GET("/quotes/random", func(c *gin.Context) {
		if len(quotes) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "No quotes available"})
			return
		}
		c.JSON(http.StatusOK, quotes[rand.Intn(len(quotes))])
	})
	
	r.GET("/quotes/:author", func(c *gin.Context) {
		author := strings.ToLower(c.Param("author"))
		var filtered []Quote
		for _, q := range quotes {
			if strings.ToLower(q.Author) == author {
				filtered = append(filtered, q)
			}
		}
		c.JSON(http.StatusOK, filtered)
	})
	
	r.Run(":" + os.Getenv("PORT"))

}

func loadQuotes() {
	file, err := os.Open("data/quotes.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&quotes); err != nil {
		panic(err)
	}
}