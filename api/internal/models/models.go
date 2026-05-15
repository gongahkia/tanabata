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

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type APIResponse[T any] struct {
	Data       T           `json:"data,omitempty"`
	Meta       any         `json:"meta,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Error      *APIError   `json:"error,omitempty"`
}

type ListResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
	Meta       ListMeta   `json:"meta"`
}

type SearchResults struct {
	Artists []Artist `json:"artists"`
	Quotes  []Quote  `json:"quotes"`
}

type SearchResponse struct {
	Data SearchResults `json:"data"`
	Meta ListMeta      `json:"meta"`
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
	ProviderOrigin   string   `json:"provider_origin,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
	License          string   `json:"license,omitempty"`
	FirstSeenAt      string   `json:"first_seen_at,omitempty"`
	LastVerifiedAt   string   `json:"last_verified_at,omitempty"`
	Source           *Source  `json:"source,omitempty"`
}

type QuoteProvenance struct {
	QuoteID          string   `json:"quote_id"`
	ProvenanceStatus string   `json:"provenance_status"`
	ConfidenceScore  float64  `json:"confidence_score"`
	ProviderOrigin   string   `json:"provider_origin"`
	FirstSeenAt      string   `json:"first_seen_at,omitempty"`
	LastVerifiedAt   string   `json:"last_verified_at,omitempty"`
	Evidence         []string `json:"evidence"`
	Source           *Source  `json:"source,omitempty"`
}

type ReviewQueueItem struct {
	Quote     Quote   `json:"quote"`
	Reason    string  `json:"reason"`
	RiskScore float64 `json:"risk_score"`
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

type CuratedQuoteRecord struct {
	ArtistName       string   `json:"artist_name"`
	Aliases          []string `json:"aliases,omitempty"`
	Text             string   `json:"text"`
	SourceType       string   `json:"source_type,omitempty"`
	WorkTitle        string   `json:"work_title,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	ProvenanceStatus string   `json:"provenance_status"`
	ConfidenceScore  float64  `json:"confidence_score"`
	ProviderOrigin   string   `json:"provider_origin,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
	License          string   `json:"license,omitempty"`
	FirstSeenAt      string   `json:"first_seen_at,omitempty"`
	LastVerifiedAt   string   `json:"last_verified_at,omitempty"`
	Source           *Source  `json:"source,omitempty"`
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

type ReviewQueueFilters struct {
	ProvenanceStatus string
	Limit            int
	Offset           int
}

type ArtistFilters struct {
	Query          string
	MBID           string
	WikiquoteTitle string
	Tag            string
	Limit          int
	Offset         int
}

type ProviderSummary struct {
	Provider         string `json:"provider"`
	Category         string `json:"category"`
	Enabled          bool   `json:"enabled"`
	LastStatus       string `json:"last_status,omitempty"`
	LastSuccessful   string `json:"last_successful,omitempty"`
	LastErrorAt      string `json:"last_error_at,omitempty"`
	RecentErrorCount int    `json:"recent_error_count"`
}

type ProviderRun struct {
	RunID      string `json:"run_id"`
	Provider   string `json:"provider"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Details    string `json:"details,omitempty"`
}

type ProviderError struct {
	ErrorID    string `json:"error_id"`
	Provider   string `json:"provider"`
	OccurredAt string `json:"occurred_at"`
	Context    string `json:"context,omitempty"`
	Message    string `json:"message"`
}

type JobRun struct {
	JobID        string    `json:"job_id"`
	Name         string    `json:"name"`
	Scope        string    `json:"scope,omitempty"`
	Status       string    `json:"status"`
	StartedAt    string    `json:"started_at"`
	FinishedAt   string    `json:"finished_at,omitempty"`
	Details      string    `json:"details,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Items        []JobItem `json:"items,omitempty"`
}

type JobItem struct {
	JobItemID    string `json:"job_item_id"`
	JobID        string `json:"job_id"`
	Provider     string `json:"provider"`
	Target       string `json:"target,omitempty"`
	Status       string `json:"status"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at,omitempty"`
	Details      string `json:"details,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}
