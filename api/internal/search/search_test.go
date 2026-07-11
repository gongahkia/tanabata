package search

import (
	"strings"
	"testing"
)

func TestNormalizeText(t *testing.T) {
	got := NormalizeText("  Frank Ocean!!  ")
	if got != "frank ocean" {
		t.Fatalf("NormalizeText() = %q, want %q", got, "frank ocean")
	}
}

func TestArtistIDPrefersMBID(t *testing.T) {
	got := ArtistID("Frank Ocean", "abcd-1234")
	if got != "abcd-1234" {
		t.Fatalf("ArtistID() = %q, want mbid", got)
	}
}

func TestQuoteIDStable(t *testing.T) {
	left := QuoteID("artist", "hello world", "https://example.com")
	right := QuoteID("artist", "hello world", "https://example.com")
	if left != right {
		t.Fatalf("QuoteID() should be deterministic")
	}
}

func TestSimilarityScoreForTypos(t *testing.T) {
	if score := SimilarityScore("frnak ocean", "Frank Ocean"); score < 45 {
		t.Fatalf("SimilarityScore() = %d, expected typo match", score)
	}
}

func TestShouldMergeQuotes(t *testing.T) {
	if !ShouldMergeQuotes("Work hard in silence.", "Work hard in silence") {
		t.Fatalf("expected punctuation-only variant to merge")
	}
	if ShouldMergeQuotes("Work hard in silence.", "Work harder in private.") {
		t.Fatalf("expected distinct quote to remain separate")
	}
}

func FuzzQuoteNormalize(f *testing.F) {
	for _, seed := range []string{
		"  Frank Ocean!!  ",
		"Work hard in silence.",
		`" OR quotes MATCH "*"`,
		"日本語",
		"\u202eartist",
		"emoji 🎸 quote",
		"line\nbreak\tcontrol\x00",
		strings.Repeat("a", 10*1024),
		"",
		"   ",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10*1024 {
			t.Skip()
		}
		normalized := NormalizeText(input)
		if normalized != strings.TrimSpace(normalized) {
			t.Fatalf("NormalizeText(%q) left outer space in %q", input, normalized)
		}
		if strings.Contains(normalized, "  ") {
			t.Fatalf("NormalizeText(%q) left repeated spaces in %q", input, normalized)
		}
		if NormalizeText(normalized) != normalized {
			t.Fatalf("NormalizeText is not idempotent for %q", normalized)
		}
		if strings.Contains(QuoteFingerprint(input), " ") {
			t.Fatalf("QuoteFingerprint(%q) contains spaces", input)
		}
		if QuoteMergeScore(input, input) != 100 {
			t.Fatalf("QuoteMergeScore should self-match for %q", input)
		}
	})
}
