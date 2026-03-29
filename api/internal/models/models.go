package models

type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

type ListMeta struct {
	SnapshotVersion string   `json:"snapshot_version"`
	ActiveProviders []string `json:"active_providers"`
}

type ListResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
	Meta       ListMeta   `json:"meta"`
}

type SearchResponse struct {
	Data struct {
		Artists []Artist `json:"artists"`
		Quotes  []Quote  `json:"quotes"`
	} `json:"data"`
	Meta ListMeta `json:"meta"`
}

type Artist struct {
	ArtistID       string            `json:"artist_id"`
	Name           string            `json:"name"`
	Aliases        []string          `json:"aliases"`
	MBID           string            `json:"mbid,omitempty"`
	WikidataID     string            `json:"wikidata_id,omitempty"`
	WikiquoteTitle string            `json:"wikiquote_title,omitempty"`
	Country        string            `json:"country,omitempty"`
	LifeSpan       LifeSpan          `json:"life_span"`
	Description    string            `json:"description,omitempty"`
	BioSummary     string            `json:"bio_summary,omitempty"`
	Genres         []string          `json:"genres"`
	Links          []ArtistLink      `json:"links"`
	ProviderStatus map[string]string `json:"provider_status"`
}

type LifeSpan struct {
	Begin string `json:"begin,omitempty"`
	End   string `json:"end,omitempty"`
}

type ArtistLink struct {
	Provider   string `json:"provider"`
	Kind       string `json:"kind"`
	URL        string `json:"url"`
	ExternalID string `json:"external_id,omitempty"`
}

type Quote struct {
	QuoteID          string   `json:"quote_id"`
	Text             string   `json:"text"`
	ArtistID         string   `json:"artist_id"`
	ArtistName       string   `json:"artist_name"`
	SourceID         string   `json:"source_id,omitempty"`
	SourceType       string   `json:"source_type,omitempty"`
	WorkTitle        string   `json:"work_title,omitempty"`
	Year             *int     `json:"year,omitempty"`
	Tags             []string `json:"tags"`
	ProvenanceStatus string   `json:"provenance_status"`
	ConfidenceScore  float64  `json:"confidence_score"`
	License          string   `json:"license,omitempty"`
	FirstSeenAt      string   `json:"first_seen_at,omitempty"`
	LastVerifiedAt   string   `json:"last_verified_at,omitempty"`
	Source           *Source  `json:"source,omitempty"`
}

type Source struct {
	SourceID    string `json:"source_id"`
	Provider    string `json:"provider"`
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
	License     string `json:"license,omitempty"`
	RetrievedAt string `json:"retrieved_at,omitempty"`
}

type Release struct {
	ReleaseID string `json:"release_id"`
	Title     string `json:"title"`
	Year      *int   `json:"year,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Provider  string `json:"provider"`
	URL       string `json:"url,omitempty"`
}

type RelatedArtist struct {
	ArtistID string  `json:"artist_id"`
	Name     string  `json:"name"`
	Relation string  `json:"relation"`
	Score    float64 `json:"score"`
	Provider string  `json:"provider"`
}

type LegacyQuote struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

type QuoteFilters struct {
	Artist           string
	ArtistID         string
	Query            string
	Tag              string
	Source           string
	ProvenanceStatus string
	Limit            int
	Offset           int
	Sort             string
}

type ArtistFilters struct {
	Query          string
	MBID           string
	WikiquoteTitle string
	Tag            string
	Limit          int
	Offset         int
}
