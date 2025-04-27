// ----- required imports -----

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
)

// ----- struct definitions -----

type Quote struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

// ----- initialization code -----

var quotes []Quote

// ----- helper functions -----

func main() {
	scrapeFlag := flag.Bool("scrape", false, "Run scraper and exit")
	flag.Parse()
	if *scrapeFlag {
		runScraper()
		return
	}
	runAPI()
}

func runScraper() {
	rapperSlugs := []string{"kendrick-lamar", "tupac-shakur", "eminem"} // FUA to add more here
	var scrapedQuotes []Quote
	for _, slug := range rapperSlugs {
		url := fmt.Sprintf("https://www.brainyquote.com/authors/%s", slug)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("Error fetching %s: %v", url, err)
			continue
		}
		defer resp.Body.Close()
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			log.Printf("Error parsing %s: %v", url, err)
			continue
		}
		doc.Find(".grid-item .qkrn-content-wrapper").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Find("a[title='view quote']").Text())
			if text != "" {
				scrapedQuotes = append(scrapedQuotes, Quote{
					Author: strings.Title(strings.ReplaceAll(slug, "-", " ")),
					Text:   text,
				})
			}
		})
	}
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal(err)
	}
	file, err := os.Create("data/quotes.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(scrapedQuotes); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Successfully scraped", len(scrapedQuotes), "quotes")
}

func runAPI() {

	loadQuotes()

	r := gin.Default()

	r.GET("/quotes", func(c *gin.Context) {
		c.JSON(http.StatusOK, quotes)
	})

	r.GET("/quotes/random", func(c *gin.Context) {
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
		log.Fatal("Error opening quotes file:", err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&quotes); err != nil {
		log.Fatal("Error decoding quotes JSON:", err)
	}
}