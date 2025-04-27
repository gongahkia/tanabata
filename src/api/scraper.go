// ----- required imports -----

package scraper

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/PuerkitoBio/goquery"
    _ "github.com/lib/pq"
)

// ----- struct definitions -----

type Quote struct {
    Author string
    Text   string
}

// ----- helper functions -----

func ScrapeAndStore(dbURL string) error {
    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        return fmt.Errorf("database connection failed: %w", err)
    }
    defer db.Close()
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _, err = db.ExecContext(ctx, "DELETE FROM quotes")
    if err != nil {
        return fmt.Errorf("failed to clear old data: %w", err)
    }
    authors := []string{"kendrick-lamar", "tupac-shakur", "eminem"}
    for _, authorSlug := range authors {
        url := fmt.Sprintf("https://www.brainyquote.com/authors/%s", authorSlug)
        quotes, err := scrapeAuthor(url)
        if err != nil {
            log.Printf("Failed to scrape %s: %v", authorSlug, err)
            continue
        }
        tx, _ := db.Begin()
        stmt, _ := tx.PrepareContext(ctx, 
            "INSERT INTO quotes (author, text) VALUES ($1, $2)")
        authorName := strings.Title(strings.ReplaceAll(authorSlug, "-", " "))
        for _, q := range quotes {
            _, err := stmt.ExecContext(ctx, authorName, q.Text)
            if err != nil {
                log.Printf("Insert failed: %v", err)
            }
        }
        tx.Commit()
    }
    return nil
}

func scrapeAuthor(url string) ([]Quote, error) {
    res, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()
    doc, err := goquery.NewDocumentFromReader(res.Body)
    if err != nil {
        return nil, err
    }
    var quotes []Quote
    doc.Find(".grid-item .qkrn-content-wrapper").Each(func(i int, s *goquery.Selection) {
        text := strings.TrimSpace(s.Find("a[title='view quote']").Text())
        if text != "" {
            quotes = append(quotes, Quote{Text: text})
        }
    })
    return quotes, nil
}