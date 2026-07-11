package catalog

import (
	"strings"
	"testing"
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

func FuzzFTSQueryEscapesTokens(f *testing.F) {
	for _, seed := range []string{`foo"bar`, `foo\"bar`, `NEAR NOT`, `a" AND b`, `a\" AND b`, `日本語`, "", "   "} {
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
