package catalog

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/models"
)

func TestFTSQueryEscapesAdversarialInput(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: `foo"bar`, want: `"foo""bar"*`},
		{input: `NEAR NOT`, want: `"NEAR"* "NOT"*`},
		{input: `a" AND b`, want: `"a"""* "AND"* "b"*`},
		{input: `日本語`, want: `"日本語"*`},
		{input: "   ", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			if got := ftsQuery(tc.input); got != tc.want {
				t.Fatalf("ftsQuery() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSearchAdversarialFTSInputsReturnNoHits(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()
	for _, input := range []string{`foo"bar`, `foo\"bar`, `NEAR NOT`, `a" AND b`, `a\" AND b`, `日本語`} {
		t.Run(input, func(t *testing.T) {
			response, err := store.Search(ctx, input)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(response.Data.Artists) != 0 || len(response.Data.Quotes) != 0 {
				t.Fatalf("Search() hits artists=%d quotes=%d", len(response.Data.Artists), len(response.Data.Quotes))
			}
		})
	}
}

func FuzzFTSQueryBuilder(f *testing.F) {
	for _, seed := range []string{
		`foo"bar`,
		`foo\"bar`,
		`NEAR NOT`,
		`a" AND b`,
		`a\" AND b`,
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
		query := ftsQuery(input)
		for _, term := range strings.Fields(query) {
			if !strings.HasPrefix(term, `"`) || !strings.HasSuffix(term, `"*`) {
				t.Fatalf("unescaped FTS term %q in query %q", term, query)
			}
		}
	})
}

func FuzzArtistIDResolve(f *testing.F) {
	store, err := Open(filepath.Join(f.TempDir(), "catalog.sqlite"))
	if err != nil {
		f.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	ctx := context.Background()
	for _, artist := range []models.Artist{
		{ArtistID: "artist-frank", Name: "Frank Ocean", Aliases: []string{"Christopher Francis Ocean", "frank ocean"}},
		{ArtistID: "artist-bjork", Name: "Bjork", Aliases: []string{"Björk", "bjork"}},
		{ArtistID: "artist-sade", Name: "Sade", Aliases: []string{"Sade Adu"}},
	} {
		if err := store.UpsertArtist(ctx, artist); err != nil {
			f.Fatalf("UpsertArtist() error = %v", err)
		}
	}
	for _, seed := range []string{
		"Frank Ocean",
		"frnak ocean",
		"Björk",
		"Sade Adu",
		`" OR 1=1 --`,
		"日本語",
		"\u202eartist",
		"emoji 🎸",
		"line\nbreak\tcontrol\x00",
		strings.Repeat("x", 10*1024),
		"",
		"   ",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10*1024 {
			t.Skip()
		}
		if _, err := store.ResolveArtistID(ctx, input); err != nil {
			t.Fatalf("ResolveArtistID(%q) error = %v", input, err)
		}
	})
}
