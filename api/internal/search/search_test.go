package search

import "testing"

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
