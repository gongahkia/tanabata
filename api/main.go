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
	"context"
	"time"
	"strings"
	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"github.com/chromedp/chromedp"
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
	rapperSlugs := []string{"kendrick-lamar", "tupac-shakur", "eminem"} // FUA to add more names later
	var scrapedQuotes []Quote
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	for _, slug := range rapperSlugs {
		url := fmt.Sprintf("https://www.brainyquote.com/authors/%s", slug)
		var htmlContent string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			chromedp.Sleep(2*time.Second), 
			chromedp.ActionFunc(func(ctx context.Context) error {
				for i := 0; i < 10; i++ {
					if err := chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil).Do(ctx); err != nil {
						return err
					}
					time.Sleep(1 * time.Second)
				}
				return nil
			}),
			chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
		)
		if err != nil {
			log.Printf("Error loading %s: %v", url, err)
			continue
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
		if err != nil {
			log.Printf("Error parsing HTML for %s: %v", url, err)
			continue
		}
		authorName := strings.Title(strings.ReplaceAll(slug, "-", " "))
		doc.Find(`[title="view quote"]`).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				scrapedQuotes = append(scrapedQuotes, Quote{
					Author: authorName,
					Text:   text,
				})
				return
			}
			img := s.Find("img")
			if img.Length() > 0 {
				alt, exists := img.Attr("alt")
				if exists && strings.TrimSpace(alt) != "" {
					scrapedQuotes = append(scrapedQuotes, Quote{
						Author: authorName,
						Text:   alt,
					})
				}
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