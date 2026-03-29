package search

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

var punctuationPattern = regexp.MustCompile(`[^a-z0-9\s]+`)

func NormalizeText(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	cleaned := punctuationPattern.ReplaceAllString(trimmed, " ")
	parts := strings.Fields(cleaned)
	return strings.Join(parts, " ")
}

func Slug(input string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(input)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case !lastDash:
			builder.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "unknown"
	}
	return slug
}

func ArtistID(name, mbid string) string {
	if strings.TrimSpace(mbid) != "" {
		return strings.TrimSpace(mbid)
	}
	return "tanabata:" + Slug(name)
}

func StableHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:16])
}

func QuoteID(artistID, normalizedText, sourceURL string) string {
	return StableHash(artistID, normalizedText, sourceURL)
}

func SourceID(provider, url string) string {
	return StableHash(provider, strings.TrimSpace(url))
}

func SimilarityScore(query, candidate string) int {
	normalizedQuery := NormalizeText(query)
	normalizedCandidate := NormalizeText(candidate)
	if normalizedQuery == normalizedCandidate {
		return 100
	}
	if strings.Contains(normalizedCandidate, normalizedQuery) {
		return 90
	}
	if strings.Contains(normalizedQuery, normalizedCandidate) {
		return 80
	}
	distance := levenshtein(normalizedQuery, normalizedCandidate)
	switch {
	case distance <= 1:
		return 75
	case distance == 2:
		return 60
	case distance == 3:
		return 45
	default:
		return 0
	}
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len([]rune(b))
	}
	if len(b) == 0 {
		return len([]rune(a))
	}

	ar := []rune(a)
	br := []rune(b)
	column := make([]int, len(br)+1)
	for y := 1; y <= len(br); y++ {
		column[y] = y
	}
	for x := 1; x <= len(ar); x++ {
		column[0] = x
		lastdiag := x - 1
		for y := 1; y <= len(br); y++ {
			olddiag := column[y]
			cost := 0
			if ar[x-1] != br[y-1] {
				cost = 1
			}
			column[y] = min3(
				column[y]+1,
				column[y-1]+1,
				lastdiag+cost,
			)
			lastdiag = olddiag
		}
	}
	return column[len(br)]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}
