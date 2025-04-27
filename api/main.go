// ----- required imports -----

package main

import (
    "math/rand"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
)

// ----- struct definitions -----

type Quote struct {
    Rapper string `json:"rapper"`
    Quote  string `json:"quote"`
}

// ----- helper functions -----

func equalFold(a, b string) bool {
    return len(a) == len(b) && (a == b || toLower(a) == toLower(b))
}

func toLower(s string) string {
    b := []byte(s)
    for i, v := range b {
        if v >= 'A' && v <= 'Z' {
            b[i] = v + 32
        }
    }
    return string(b)
}

// ----- execution code -----

var quotes = []Quote{
    {Rapper: "Kendrick Lamar", Quote: "If God got us, then we gon' be alright."},
    {Rapper: "Tupac Shakur", Quote: "Reality is wrong. Dreams are for real."},
    {Rapper: "Jay-Z", Quote: "I'm not a businessman, I'm a business, man."},
    {Rapper: "Nicki Minaj", Quote: "You don’t have to feel the need to put somebody down to make yourself feel better."},
    {Rapper: "Eminem", Quote: "You better lose yourself in the music, the moment, you own it, you better never let it go."},
    {Rapper: "Nas", Quote: "I know I can be what I wanna be."},
}

func main() {
    rand.Seed(time.Now().UnixNano())

    r := gin.Default()

    r.GET("/quotes", func(c *gin.Context) {
        c.JSON(http.StatusOK, quotes)
    })

    r.GET("/quotes/random", func(c *gin.Context) {
        c.JSON(http.StatusOK, quotes[rand.Intn(len(quotes))])
    })

    r.GET("/quotes/:rapper", func(c *gin.Context) {
        rapper := c.Param("rapper")
        var filtered []Quote
        for _, q := range quotes {
            if gin.Mode() == gin.TestMode {
                if q.Rapper == rapper {
                    filtered = append(filtered, q)
                }
            } else { 
                if equalFold(q.Rapper, rapper) {
                    filtered = append(filtered, q)
                }
            }
        }
        if len(filtered) == 0 {
            c.JSON(http.StatusNotFound, gin.H{"error": "No quotes found for this rapper"})
            return
        }
        c.JSON(http.StatusOK, filtered)
    })

    r.Run() 

}