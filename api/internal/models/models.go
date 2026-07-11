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

type CursorMeta struct {
	SnapshotVersion string   `json:"snapshot_version"`
	ActiveProviders []string `json:"active_providers"`
	NextCursor      string   `json:"next_cursor,omitempty"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type ProblemDetails struct {
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Status   int            `json:"status"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
	Code     string         `json:"code"`
	Details  map[string]any `json:"details,omitempty"`
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

type EntitySearchHit struct {
	Kind    string  `json:"kind"`
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet,omitempty"`
}

type EntitySearchResults struct {
	Hits []EntitySearchHit `json:"hits"`
}

type SearchResponse struct {
	Data SearchResults `json:"data"`
	Meta ListMeta      `json:"meta"`
}

type EntitySearchResponse struct {
	Data EntitySearchResults `json:"data"`
	Meta ListMeta            `json:"meta"`
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
	FreshnessStatus  string   `json:"freshness_status,omitempty"`
	FreshnessAgeDays *int     `json:"freshness_age_days,omitempty"`
	FreshnessReason  string   `json:"freshness_reason,omitempty"`
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

type SimilarQuote struct {
	Quote      Quote   `json:"quote"`
	MergeScore float64 `json:"merge_score"`
}

type ArtistProvenanceSummary struct {
	ArtistID            string         `json:"artist_id"`
	StatusCounts        map[string]int `json:"status_counts"`
	ConfidenceHistogram []int          `json:"confidence_histogram"`
	MeanConfidence      float64        `json:"mean_confidence"`
	RefreshHint         string         `json:"refresh_hint"`
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
	FreshnessStatus  string
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
	CooldownUntil    string `json:"cooldown_until,omitempty"`
	CooldownReason   string `json:"cooldown_reason,omitempty"`
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
	Kind       string `json:"kind,omitempty"`
	OccurredAt string `json:"occurred_at"`
	Context    string `json:"context,omitempty"`
	Message    string `json:"message"`
}

type ProviderCooldown struct {
	Provider  string `json:"provider"`
	Until     string `json:"until"`
	Reason    string `json:"reason,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

type JobRun struct {
	JobID        string                `json:"job_id"`
	Name         string                `json:"name"`
	Scope        string                `json:"scope,omitempty"`
	Status       string                `json:"status"`
	StartedAt    string                `json:"started_at"`
	FinishedAt   string                `json:"finished_at,omitempty"`
	Details      string                `json:"details,omitempty"`
	ErrorMessage string                `json:"error_message,omitempty"`
	Items        []JobItem             `json:"items,omitempty"`
	Snapshots    []IngestionSnapshot   `json:"snapshots,omitempty"`
	AuditEvents  []IngestionAuditEvent `json:"audit_events,omitempty"`
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

type IngestionSnapshot struct {
	SnapshotID string         `json:"snapshot_id"`
	JobID      string         `json:"job_id,omitempty"`
	Phase      string         `json:"phase"`
	CapturedAt string         `json:"captured_at"`
	Counts     map[string]int `json:"counts"`
}

type IngestionAuditEvent struct {
	EventID    string `json:"event_id"`
	JobID      string `json:"job_id,omitempty"`
	JobItemID  string `json:"job_item_id,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Target     string `json:"target,omitempty"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	OccurredAt string `json:"occurred_at"`
	Details    string `json:"details,omitempty"`
}

type WebhookSubscription struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Secret        string   `json:"secret,omitempty"`
	EventKinds    []string `json:"event_kinds"`
	CreatedAt     string   `json:"created_at"`
	DisabledAt    string   `json:"disabled_at,omitempty"`
	LastSuccessAt string   `json:"last_success_at,omitempty"`
	FailureCount  int      `json:"failure_count"`
}

type WebhookEvent struct {
	EventID    string `json:"event_id"`
	Kind       string `json:"kind"`
	OccurredAt string `json:"occurred_at"`
	Data       any    `json:"data"`
}

type TimelineEvent struct {
	EventID  string         `json:"event_id"`
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	Status   string         `json:"status,omitempty"`
	At       string         `json:"at"`
	Details  string         `json:"details,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type IntegrityReport struct {
	OK        bool           `json:"ok"`
	CheckedAt string         `json:"checked_at"`
	SQLite    string         `json:"sqlite"`
	Counts    map[string]int `json:"counts"`
	Issues    []string       `json:"issues"`
}

// Work is a composition (an abstract song), distinct from any particular Recording of it.
type Work struct {
	WorkID            string   `json:"work_id"`
	MBID              string   `json:"mbid,omitempty"`
	Title             string   `json:"title"`
	ISWC              string   `json:"iswc,omitempty"`
	Language          string   `json:"language,omitempty"`
	CreatedYear       string   `json:"created_year,omitempty"`
	PrimaryArtistID   string   `json:"primary_artist_id,omitempty"`
	PrimaryArtistName string   `json:"primary_artist_name,omitempty"`
	Notes             string   `json:"notes,omitempty"`
	RecordingCount    int      `json:"recording_count"`
	CreditCount       int      `json:"credit_count"`
	DisputedCredits   int      `json:"disputed_credit_count"`
	Tags              []string `json:"tags,omitempty"`
}

// Recording is a specific recorded performance of a Work.
type Recording struct {
	RecordingID  string `json:"recording_id"`
	MBID         string `json:"mbid,omitempty"`
	WorkID       string `json:"work_id,omitempty"`
	WorkTitle    string `json:"work_title,omitempty"`
	ArtistID     string `json:"artist_id"`
	ArtistName   string `json:"artist_name"`
	Title        string `json:"title"`
	DurationMs   int    `json:"duration_ms,omitempty"`
	ReleasedYear string `json:"released_year,omitempty"`
	ReleaseID    string `json:"release_id,omitempty"`
	ISRC         string `json:"isrc,omitempty"`
	IsOriginal   bool   `json:"is_original"`
	Notes        string `json:"notes,omitempty"`
	SampleOutDeg int    `json:"sample_outgoing_count"`
	SampleInDeg  int    `json:"sample_incoming_count"`
}

// SampleEdge represents recording_a sampling recording_b, with optional timestamps.
type SampleEdge struct {
	SampleID            string     `json:"sample_id"`
	SourceRecording     Recording  `json:"source_recording"`
	DerivativeRecording Recording  `json:"derivative_recording"`
	Kind                string     `json:"kind"`
	SourceOffsetMs      int        `json:"source_offset_ms,omitempty"`
	DerivativeOffsetMs  int        `json:"derivative_offset_ms,omitempty"`
	DurationMs          int        `json:"duration_ms,omitempty"`
	Notes               string     `json:"notes,omitempty"`
	Claim               *ClaimView `json:"claim,omitempty"`
}

// WorkCredit attributes a contributor to a work in some role.
type WorkCredit struct {
	CreditID         string     `json:"credit_id"`
	WorkID           string     `json:"work_id"`
	CreditedArtistID string     `json:"credited_artist_id,omitempty"`
	CreditedName     string     `json:"credited_name"`
	Role             string     `json:"role"`
	IsDisputed       bool       `json:"is_disputed"`
	Notes            string     `json:"notes,omitempty"`
	Claim            *ClaimView `json:"claim,omitempty"`
}

// Performance is a live appearance: artist played work/recording at venue on a date.
type Performance struct {
	PerformanceID string     `json:"performance_id"`
	ArtistID      string     `json:"artist_id"`
	ArtistName    string     `json:"artist_name"`
	WorkID        string     `json:"work_id,omitempty"`
	WorkTitle     string     `json:"work_title,omitempty"`
	RecordingID   string     `json:"recording_id,omitempty"`
	EventName     string     `json:"event_name,omitempty"`
	Venue         string     `json:"venue,omitempty"`
	City          string     `json:"city,omitempty"`
	Country       string     `json:"country,omitempty"`
	PerformedAt   string     `json:"performed_at"`
	SetlistFMID   string     `json:"setlistfm_id,omitempty"`
	PositionInSet int        `json:"position_in_set,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	Claim         *ClaimView `json:"claim,omitempty"`
}

// PerformanceStats summarises how a work has appeared in an artist's live history.
type PerformanceStats struct {
	ArtistID         string  `json:"artist_id"`
	WorkID           string  `json:"work_id,omitempty"`
	WorkTitle        string  `json:"work_title,omitempty"`
	TotalPerformed   int     `json:"total_performed"`
	FirstPerformedAt string  `json:"first_performed_at,omitempty"`
	LastPerformedAt  string  `json:"last_performed_at,omitempty"`
	GapDays          int     `json:"gap_days,omitempty"`
	AverageGapDays   float64 `json:"average_gap_days,omitempty"`
	Venues           int     `json:"distinct_venues"`
	Countries        int     `json:"distinct_countries"`
}

// Claim is the unifying provenance unit across all five feature families.
type Claim struct {
	ClaimID         string          `json:"claim_id"`
	Kind            string          `json:"kind"`
	SubjectType     string          `json:"subject_type"`
	SubjectID       string          `json:"subject_id"`
	ObjectType      string          `json:"object_type"`
	ObjectID        string          `json:"object_id"`
	Relation        string          `json:"relation,omitempty"`
	Status          string          `json:"status"`
	ConfidenceScore float64         `json:"confidence_score"`
	ProviderOrigin  string          `json:"provider_origin,omitempty"`
	SourceID        string          `json:"source_id,omitempty"`
	AssertedAt      string          `json:"asserted_at"`
	LastVerifiedAt  string          `json:"last_verified_at,omitempty"`
	UpdatedAt       string          `json:"updated_at,omitempty"`
	Notes           string          `json:"notes,omitempty"`
	Source          *Source         `json:"source,omitempty"`
	SupportingCount int             `json:"supporting_evidence_count"`
	RefutingCount   int             `json:"refuting_evidence_count"`
	Evidence        []ClaimEvidence `json:"evidence,omitempty"`
}

// ClaimView is an embedded snapshot of the claim attached to relationship rows.
type ClaimView struct {
	ClaimID         string  `json:"claim_id"`
	Status          string  `json:"status"`
	ConfidenceScore float64 `json:"confidence_score"`
	ProviderOrigin  string  `json:"provider_origin,omitempty"`
	SupportingCount int     `json:"supporting_evidence_count"`
	RefutingCount   int     `json:"refuting_evidence_count"`
}

type GraphNode struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type GraphEdge struct {
	From            string  `json:"from"`
	To              string  `json:"to"`
	Kind            string  `json:"kind"`
	ClaimID         string  `json:"claim_id"`
	Status          string  `json:"status"`
	ConfidenceScore float64 `json:"confidence_score"`
}

type EntityGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// ClaimEvidence supports or refutes a Claim.
type ClaimEvidence struct {
	EvidenceID   string  `json:"evidence_id"`
	ClaimID      string  `json:"claim_id"`
	Supports     bool    `json:"supports"`
	SourceID     string  `json:"source_id,omitempty"`
	Excerpt      string  `json:"excerpt"`
	SourceURL    string  `json:"source_url,omitempty"`
	ArchivedURL  string  `json:"archived_url,omitempty"`
	EvidenceKind string  `json:"evidence_kind"`
	Weight       float64 `json:"weight"`
	RecordedAt   string  `json:"recorded_at"`
	Source       *Source `json:"source,omitempty"`
}

// QuoteLineage is the chronologically ordered trail of evidence (and counter-evidence) for a quote attribution.
type QuoteLineage struct {
	QuoteID            string          `json:"quote_id"`
	Text               string          `json:"text"`
	AttributedToID     string          `json:"attributed_to_id"`
	AttributedToName   string          `json:"attributed_to_name"`
	ProvenanceStatus   string          `json:"provenance_status"`
	ConfidenceScore    float64         `json:"confidence_score"`
	EarliestEvidenceAt string          `json:"earliest_evidence_at,omitempty"`
	LatestEvidenceAt   string          `json:"latest_evidence_at,omitempty"`
	Supporting         []ClaimEvidence `json:"supporting_evidence"`
	Refuting           []ClaimEvidence `json:"refuting_evidence"`
	RivalClaims        []Claim         `json:"rival_claims,omitempty"`
	MergeHistory       []QuoteMergeLog `json:"merge_history,omitempty"`
}

// QuoteMergeLog records that loser_quote_id was merged into winner_quote_id (closes ADR-004's audit gap).
type QuoteMergeLog struct {
	MergeID       string `json:"merge_id"`
	WinnerQuoteID string `json:"winner_quote_id"`
	LoserQuoteID  string `json:"loser_quote_id"`
	MergeScore    int    `json:"merge_score"`
	Reason        string `json:"reason"`
	MergedAt      string `json:"merged_at"`
	JobID         string `json:"job_id,omitempty"`
}

// Dispute is a claim whose status is disputed/ambiguous/refuted, surfaced cross-cutting.
type Dispute struct {
	Claim            Claim  `json:"claim"`
	SubjectLabel     string `json:"subject_label,omitempty"`
	ObjectLabel      string `json:"object_label,omitempty"`
	HumanDescription string `json:"human_description,omitempty"`
}

// Filters for new surfaces.

type RecordingFilters struct {
	ArtistID string
	WorkID   string
	Query    string
	Limit    int
	Offset   int
}

type WorkFilters struct {
	Query    string
	ArtistID string
	Limit    int
	Offset   int
}

type SampleFilters struct {
	Kind  string
	Limit int
}

type PerformanceFilters struct {
	ArtistID string
	WorkID   string
	Year     string
	Limit    int
	Offset   int
	Sort     string
}

type ClaimFilters struct {
	Kind   string
	Status string
	Limit  int
	Offset int
}

// Curated bundles for ingestion.

type CuratedSampleRecord struct {
	SourceArtistName       string            `json:"source_artist_name"`
	SourceTrackTitle       string            `json:"source_track_title"`
	SourceReleasedYear     string            `json:"source_released_year,omitempty"`
	DerivativeArtistName   string            `json:"derivative_artist_name"`
	DerivativeTrackTitle   string            `json:"derivative_track_title"`
	DerivativeReleasedYear string            `json:"derivative_released_year,omitempty"`
	Kind                   string            `json:"kind"`
	SourceOffsetMs         int               `json:"source_offset_ms,omitempty"`
	DerivativeOffsetMs     int               `json:"derivative_offset_ms,omitempty"`
	DurationMs             int               `json:"duration_ms,omitempty"`
	Notes                  string            `json:"notes,omitempty"`
	Status                 string            `json:"status"`
	ConfidenceScore        float64           `json:"confidence_score"`
	ProviderOrigin         string            `json:"provider_origin,omitempty"`
	Evidence               []CuratedEvidence `json:"evidence,omitempty"`
}

type CuratedWorkRecord struct {
	Title             string                `json:"title"`
	PrimaryArtistName string                `json:"primary_artist_name"`
	CreatedYear       string                `json:"created_year,omitempty"`
	ISWC              string                `json:"iswc,omitempty"`
	Language          string                `json:"language,omitempty"`
	Notes             string                `json:"notes,omitempty"`
	Credits           []CuratedCreditRecord `json:"credits,omitempty"`
	Covers            []CuratedCoverRecord  `json:"covers,omitempty"`
}

type CuratedCreditRecord struct {
	CreditedName    string            `json:"credited_name"`
	Role            string            `json:"role"`
	IsDisputed      bool              `json:"is_disputed,omitempty"`
	Notes           string            `json:"notes,omitempty"`
	Status          string            `json:"status"`
	ConfidenceScore float64           `json:"confidence_score"`
	ProviderOrigin  string            `json:"provider_origin,omitempty"`
	Evidence        []CuratedEvidence `json:"evidence,omitempty"`
}

type CuratedCoverRecord struct {
	ArtistName      string            `json:"artist_name"`
	RecordingTitle  string            `json:"recording_title,omitempty"`
	ReleasedYear    string            `json:"released_year,omitempty"`
	IsOriginal      bool              `json:"is_original,omitempty"`
	Notes           string            `json:"notes,omitempty"`
	Status          string            `json:"status"`
	ConfidenceScore float64           `json:"confidence_score"`
	ProviderOrigin  string            `json:"provider_origin,omitempty"`
	Evidence        []CuratedEvidence `json:"evidence,omitempty"`
}

type CuratedPerformanceRecord struct {
	ArtistName      string            `json:"artist_name"`
	WorkTitle       string            `json:"work_title,omitempty"`
	EventName       string            `json:"event_name,omitempty"`
	Venue           string            `json:"venue,omitempty"`
	City            string            `json:"city,omitempty"`
	Country         string            `json:"country,omitempty"`
	PerformedAt     string            `json:"performed_at"`
	PositionInSet   int               `json:"position_in_set,omitempty"`
	SetlistFMID     string            `json:"setlistfm_id,omitempty"`
	Notes           string            `json:"notes,omitempty"`
	Status          string            `json:"status"`
	ConfidenceScore float64           `json:"confidence_score"`
	ProviderOrigin  string            `json:"provider_origin,omitempty"`
	Evidence        []CuratedEvidence `json:"evidence,omitempty"`
}

type CuratedMisquoteRecord struct {
	AttributedToName   string            `json:"attributed_to_name"`
	Text               string            `json:"text"`
	Tags               []string          `json:"tags,omitempty"`
	Status             string            `json:"status"`
	ConfidenceScore    float64           `json:"confidence_score"`
	ProviderOrigin     string            `json:"provider_origin,omitempty"`
	License            string            `json:"license,omitempty"`
	Notes              string            `json:"notes,omitempty"`
	SupportingEvidence []CuratedEvidence `json:"supporting_evidence,omitempty"`
	RefutingEvidence   []CuratedEvidence `json:"refuting_evidence,omitempty"`
	ActuallySaidByName string            `json:"actually_said_by_name,omitempty"`
}

type CuratedEvidence struct {
	Excerpt      string  `json:"excerpt"`
	SourceURL    string  `json:"source_url,omitempty"`
	ArchivedURL  string  `json:"archived_url,omitempty"`
	EvidenceKind string  `json:"evidence_kind,omitempty"`
	Weight       float64 `json:"weight,omitempty"`
	RecordedAt   string  `json:"recorded_at,omitempty"`
	Publisher    string  `json:"publisher,omitempty"`
	License      string  `json:"license,omitempty"`
}
