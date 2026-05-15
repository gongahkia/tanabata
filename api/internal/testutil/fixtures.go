package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/models"
)

func LegacyQuotes() []models.LegacyQuote {
	return []models.LegacyQuote{
		{Author: "Frank Ocean", Text: "Work hard in silence."},
		{Author: "Frank Ocean", Text: "Be yourself."},
		{Author: "Taylor Swift", Text: "Just keep dancing."},
	}
}

func CuratedQuotes() []models.CuratedQuoteRecord {
	return []models.CuratedQuoteRecord{
		{
			ArtistName:       "Frank Ocean",
			Aliases:          []string{"Christopher Francis Ocean"},
			Text:             "Make the work precise enough that it can survive your mood.",
			SourceType:       "editorial_archive",
			WorkTitle:        "Studio Notes",
			Tags:             []string{"craft", "process"},
			ProvenanceStatus: "verified",
			ConfidenceScore:  0.99,
			ProviderOrigin:   "tanabata_curated",
			Evidence: []string{
				"Curated Tanabata editorial note matched to a maintained archive entry.",
				"Reviewed against source snapshot on 2026-04-01.",
			},
			License:        "editorial_excerpt",
			FirstSeenAt:    "2026-04-01T00:00:00Z",
			LastVerifiedAt: "2026-04-01T00:00:00Z",
			Source: &models.Source{
				Provider:    "editorial_archive",
				URL:         "https://archive.tanabata.dev/frank-ocean/studio-notes",
				Title:       "Frank Ocean Studio Notes",
				Publisher:   "Tanabata Archive",
				License:     "editorial_excerpt",
				RetrievedAt: "2026-04-01T00:00:00Z",
			},
		},
		{
			ArtistName:       "Frank Ocean",
			Text:             "Make the work precise enough that it can survive your mood",
			SourceType:       "oral_history_transcript",
			WorkTitle:        "Oral History Transcript",
			Tags:             []string{"craft", "process"},
			ProvenanceStatus: "source_attributed",
			ConfidenceScore:  0.93,
			ProviderOrigin:   "tanabata_curated",
			Evidence: []string{
				"Matched against a second oral history transcript capture.",
				"Secondary archive confirms wording with punctuation variance only.",
			},
			License:        "editorial_excerpt",
			FirstSeenAt:    "2026-04-01T12:00:00Z",
			LastVerifiedAt: "2026-04-01T12:00:00Z",
			Source: &models.Source{
				Provider:    "oral_history_archive",
				URL:         "https://archive.tanabata.dev/frank-ocean/oral-history",
				Title:       "Frank Ocean Oral History Transcript",
				Publisher:   "Tanabata Archive",
				License:     "editorial_excerpt",
				RetrievedAt: "2026-04-01T12:00:00Z",
			},
		},
		{
			ArtistName:       "Taylor Swift",
			Text:             "The part people keep is usually the part that felt dangerous to say.",
			SourceType:       "editorial_archive",
			WorkTitle:        "Interview Archive",
			Tags:             []string{"writing"},
			ProvenanceStatus: "source_attributed",
			ConfidenceScore:  0.91,
			ProviderOrigin:   "tanabata_curated",
			Evidence: []string{
				"Attributed through a maintained interview source record.",
				"Source URL stored with retrieval timestamp.",
			},
			License:        "editorial_excerpt",
			FirstSeenAt:    "2026-04-02T00:00:00Z",
			LastVerifiedAt: "2026-04-02T00:00:00Z",
			Source: &models.Source{
				Provider:    "interview_archive",
				URL:         "https://archive.tanabata.dev/taylor-swift/interview-archive",
				Title:       "Taylor Swift Interview Archive",
				Publisher:   "Tanabata Archive",
				License:     "editorial_excerpt",
				RetrievedAt: "2026-04-02T00:00:00Z",
			},
		},
		{
			ArtistName:       "Taylor Swift",
			Text:             "The part people keep is usually the part that felt dangerous to say",
			SourceType:       "profile_archive",
			WorkTitle:        "Profile Archive",
			Tags:             []string{"writing"},
			ProvenanceStatus: "verified",
			ConfidenceScore:  0.96,
			ProviderOrigin:   "tanabata_curated",
			Evidence: []string{
				"Long-form profile archive repeats the same wording.",
				"Promoted to verified after cross-source review between interview and profile records.",
			},
			License:        "editorial_excerpt",
			FirstSeenAt:    "2026-04-02T12:00:00Z",
			LastVerifiedAt: "2026-04-03T00:00:00Z",
			Source: &models.Source{
				Provider:    "profile_archive",
				URL:         "https://archive.tanabata.dev/taylor-swift/profile-archive",
				Title:       "Taylor Swift Profile Archive",
				Publisher:   "Tanabata Archive",
				License:     "editorial_excerpt",
				RetrievedAt: "2026-04-02T12:00:00Z",
			},
		},
		{
			ArtistName:       "Taylor Swift",
			Text:             "If the room leans forward, the line usually has a pulse.",
			SourceType:       "session_archive",
			WorkTitle:        "Session Archive",
			Tags:             []string{"writing", "performance"},
			ProvenanceStatus: "source_attributed",
			ConfidenceScore:  0.89,
			ProviderOrigin:   "tanabata_curated",
			Evidence: []string{
				"Captured from a separate session archive with full source metadata.",
			},
			License:        "editorial_excerpt",
			FirstSeenAt:    "2026-04-03T12:00:00Z",
			LastVerifiedAt: "2026-04-03T12:00:00Z",
			Source: &models.Source{
				Provider:    "session_archive",
				URL:         "https://archive.tanabata.dev/taylor-swift/session-archive",
				Title:       "Taylor Swift Session Archive",
				Publisher:   "Tanabata Archive",
				License:     "editorial_excerpt",
				RetrievedAt: "2026-04-03T12:00:00Z",
			},
		},
		{
			ArtistName:       "Mitski",
			Text:             "You can hear when a song is trying to be brave instead of simply being honest.",
			SourceType:       "provider_digest",
			WorkTitle:        "Provider Digest",
			Tags:             []string{"honesty"},
			ProvenanceStatus: "provider_attributed",
			ConfidenceScore:  0.72,
			ProviderOrigin:   "tanabata_curated",
			Evidence: []string{
				"Imported from a provider-only digest with no durable public source URL.",
			},
			License:        "provider_digest",
			FirstSeenAt:    "2026-04-03T00:00:00Z",
			LastVerifiedAt: "2026-04-03T00:00:00Z",
		},
		{
			ArtistName:       "Tyler, The Creator",
			Text:             "If the quote keeps changing between sources, the disagreement is part of the record.",
			SourceType:       "conflict_note",
			WorkTitle:        "Conflict Review",
			Tags:             []string{"review"},
			ProvenanceStatus: "ambiguous",
			ConfidenceScore:  0.41,
			ProviderOrigin:   "tanabata_curated",
			Evidence: []string{
				"Two archived captures disagree on wording.",
				"Kept for review coverage rather than surfaced as fully verified.",
			},
			License:        "review_note",
			FirstSeenAt:    "2026-04-04T00:00:00Z",
			LastVerifiedAt: "2026-04-04T00:00:00Z",
			Source: &models.Source{
				Provider:    "conflict_review_archive",
				URL:         "https://archive.tanabata.dev/tyler-the-creator/conflict-review",
				Title:       "Tyler, The Creator Conflict Review",
				Publisher:   "Tanabata Archive",
				License:     "review_note",
				RetrievedAt: "2026-04-04T00:00:00Z",
			},
		},
	}
}

func WriteLegacyQuotes(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "quotes.json")
	writeJSON(t, path, LegacyQuotes())
	return path
}

func WriteCuratedQuotes(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "curated_quotes.json")
	writeJSON(t, path, CuratedQuotes())
	return path
}

func writeJSON(t *testing.T, path string, payload any) {
	t.Helper()
	content, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
