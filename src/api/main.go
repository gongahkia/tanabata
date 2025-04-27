// ----- required imports -----

package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"
    "time"
    "github.com/gin-gonic/gin"
    _ "github.com/lib/pq"
)

// ----- struct definitions -----

type Quote struct {
    ID       int    `json:"id"`
    Author   string `json:"author"`
    Text     string `json:"text"`
}

// ----- execution code -----

func main() {
    db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatal("Database connection failed:", err)
    }
    defer db.Close()

    r := gin.Default()

    r.GET("/quotes", func(c *gin.Context) {
        rows, err := db.Query(`
            SELECT id, author, text 
            FROM quotes 
            ORDER BY scraped_at DESC
        `)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        defer rows.Close()
        var quotes []Quote
        for rows.Next() {
            var q Quote
            rows.Scan(&q.ID, &q.Author, &q.Text)
            quotes = append(quotes, q)
        }
        c.JSON(http.StatusOK, quotes)
    })

    r.GET("/quotes/:author", func(c *gin.Context) {
        author := strings.ToLower(c.Param("author"))
        rows, err := db.Query(`
            SELECT id, author, text 
            FROM quotes 
            WHERE LOWER(author) = $1
        `, author)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        defer rows.Close()
        var quotes []Quote
        for rows.Next() {
            var q Quote
            rows.Scan(&q.ID, &q.Author, &q.Text)
            quotes = append(quotes, q)
        }
        c.JSON(http.StatusOK, quotes)
    })

    r.Run(":" + os.Getenv("PORT"))

}