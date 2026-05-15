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

func QuoteFingerprint(input string) string {
	return strings.ReplaceAll(NormalizeText(input), " ", "")
}

func QuoteMergeScore(left, right string) int {
	normalizedLeft := NormalizeText(left)
	normalizedRight := NormalizeText(right)
	if normalizedLeft == normalizedRight {
		return 100
	}
	if QuoteFingerprint(left) == QuoteFingerprint(right) {
		return 98
	}

	score := SimilarityScore(left, right)
	leftTokens := strings.Fields(normalizedLeft)
	rightTokens := strings.Fields(normalizedRight)
	if overlap := tokenOverlap(leftTokens, rightTokens); overlap >= 0.95 {
		score = maxInt(score, 95)
	} else if overlap >= 0.85 && absInt(len(leftTokens)-len(rightTokens)) <= 1 {
		score = maxInt(score, 90)
	}
	return score
}

func ShouldMergeQuotes(left, right string) bool {
	return QuoteMergeScore(left, right) >= 90
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

func tokenOverlap(left, right []string) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	counts := map[string]int{}
	for _, token := range left {
		counts[token]++
	}
	matches := 0
	for _, token := range right {
		if counts[token] > 0 {
			counts[token]--
			matches++
		}
	}
	denominator := len(left)
	if len(right) > denominator {
		denominator = len(right)
	}
	return float64(matches) / float64(denominator)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
