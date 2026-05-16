/* eslint-disable */
// This file is generated from openapi/openapi.json. Do not edit by hand.

export interface Pagination { "limit"?: number; "offset"?: number; "total"?: number; }

export interface ListMeta { "snapshot_version"?: string; "active_providers"?: string[]; }

export interface LifeSpan { "begin"?: string; "end"?: string; }

export interface ArtistLink { "provider"?: string; "kind"?: string; "url"?: string; "external_id"?: string; }

export interface Artist { "artist_id"?: string; "name"?: string; "aliases"?: string[]; "mbid"?: string; "wikidata_id"?: string; "wikiquote_title"?: string; "country"?: string; "life_span"?: LifeSpan; "description"?: string; "bio_summary"?: string; "genres"?: string[]; "links"?: ArtistLink[]; "provider_status"?: Record<string, string>; }

export interface Source { "source_id"?: string; "provider"?: string; "url"?: string; "title"?: string; "publisher"?: string; "license"?: string; "retrieved_at"?: string; }

export interface Quote { "quote_id"?: string; "text"?: string; "artist_id"?: string; "artist_name"?: string; "source_id"?: string; "source_type"?: string; "work_title"?: string; "year"?: number; "tags"?: string[]; "provenance_status"?: string; "confidence_score"?: number; "provider_origin"?: string; "evidence"?: string[]; "license"?: string; "first_seen_at"?: string; "last_verified_at"?: string; "freshness_status"?: "fresh" | "aging" | "stale" | "unknown"; "freshness_age_days"?: number; "freshness_reason"?: string; "source"?: Source; }

export interface QuoteProvenance { "quote_id"?: string; "provenance_status"?: string; "confidence_score"?: number; "provider_origin"?: string; "first_seen_at"?: string; "last_verified_at"?: string; "evidence"?: string[]; "source"?: Source; }

export interface ReviewQueueItem { "quote"?: Quote; "reason"?: string; "risk_score"?: number; }

export interface Release { "release_id"?: string; "title"?: string; "year"?: number; "kind"?: string; "provider"?: string; "url"?: string; }

export interface RelatedArtist { "artist_id"?: string; "name"?: string; "relation"?: string; "score"?: number; "provider"?: string; }

export interface ProviderSummary { "provider"?: string; "category"?: string; "enabled"?: boolean; "last_status"?: string; "last_successful"?: string; "last_error_at"?: string; "recent_error_count"?: number; "cooldown_until"?: string; "cooldown_reason"?: string; }

export interface ProviderRun { "run_id"?: string; "provider"?: string; "status"?: string; "started_at"?: string; "finished_at"?: string; "details"?: string; }

export interface ProviderError { "error_id"?: string; "provider"?: string; "occurred_at"?: string; "context"?: string; "message"?: string; }

export interface JobItem { "job_item_id"?: string; "job_id"?: string; "provider"?: string; "target"?: string; "status"?: string; "started_at"?: string; "finished_at"?: string; "details"?: string; "error_message"?: string; }

export interface JobRun { "job_id"?: string; "name"?: string; "scope"?: string; "status"?: string; "started_at"?: string; "finished_at"?: string; "details"?: string; "error_message"?: string; "items"?: JobItem[]; }

export interface SearchResults { "artists"?: Artist[]; "quotes"?: Quote[]; }

export interface IntegrityReport { "ok"?: boolean; "checked_at"?: string; "sqlite"?: string; "counts"?: Record<string, number>; "issues"?: string[]; }

export interface LyricsResult { "provider"?: string; "artist"?: string; "track"?: string; "lyrics"?: string; "synced_lyrics"?: string; "source_url"?: string; }

export interface SetlistArtist { "name"?: string; "mbid"?: string; }

export interface SetlistVenueCountry { "name"?: string; }

export interface SetlistVenueCity { "name"?: string; "country"?: SetlistVenueCountry; }

export interface SetlistVenue { "name"?: string; "city"?: SetlistVenueCity; }

export interface Setlist { "id"?: string; "eventDate"?: string; "url"?: string; "artist"?: SetlistArtist; "venue"?: SetlistVenue; "sets"?: Record<string, unknown>; "lastUpdated"?: string; }

export interface listArtistsParams {
  "q"?: string;
  "mbid"?: string;
  "wikiquote_title"?: string;
  "tag"?: string;
  "limit"?: number;
  "offset"?: number;
}

export interface getArtistParams {
  "artist_id": string;
}

export interface getArtistQuotesParams {
  "artist_id": string;
  "q"?: string;
  "tag"?: string;
  "source"?: string;
  "provenance_status"?: "verified" | "source_attributed" | "provider_attributed" | "ambiguous" | "needs_review";
  "freshness_status"?: "fresh" | "aging" | "stale" | "unknown";
  "limit"?: number;
  "offset"?: number;
}

export interface getArtistRelatedParams {
  "artist_id": string;
}

export interface getArtistReleasesParams {
  "artist_id": string;
}

export interface getArtistSetlistsParams {
  "artist_id": string;
}

export interface listQuotesParams {
  "artist"?: string;
  "artist_id"?: string;
  "q"?: string;
  "tag"?: string;
  "source"?: string;
  "provenance_status"?: "verified" | "source_attributed" | "provider_attributed" | "ambiguous" | "needs_review";
  "freshness_status"?: "fresh" | "aging" | "stale" | "unknown";
  "sort"?: string;
  "limit"?: number;
  "offset"?: number;
}

export interface getRandomQuoteParams {
  "artist"?: string;
  "artist_id"?: string;
  "q"?: string;
  "tag"?: string;
  "source"?: string;
  "provenance_status"?: "verified" | "source_attributed" | "provider_attributed" | "ambiguous" | "needs_review";
}

export interface getQuoteParams {
  "quote_id": string;
}

export interface getQuoteProvenanceParams {
  "quote_id": string;
}

export interface getSourceParams {
  "source_id": string;
}

export interface listProviderRunsParams {
  "provider": string;
  "limit"?: number;
}

export interface listProviderErrorsParams {
  "provider": string;
  "limit"?: number;
}

export interface listJobsParams {
  "limit"?: number;
}

export interface getJobParams {
  "job_id": string;
}

export interface listReviewQueueParams {
  "provenance_status"?: "provider_attributed" | "ambiguous" | "needs_review";
  "limit"?: number;
  "offset"?: number;
}

export interface listStaleQuotesParams {
  "limit"?: number;
  "offset"?: number;
}

export interface searchCatalogParams {
  "q": string;
}

export interface getLyricsParams {
  "artist": string;
  "track": string;
  "provider"?: "auto" | "lrclib" | "lyricsovh";
}

type QueryParams = Record<string, string | number | boolean | undefined>;

export interface ClientConfig {
  baseUrl?: string;
  fetchImpl?: typeof fetch;
}

async function request<T>(
  fetchImpl: typeof fetch,
  baseUrl: string,
  route: string,
  query: QueryParams | undefined,
  init?: RequestInit
): Promise<T> {
  const url = new URL(route, baseUrl.endsWith("/") ? baseUrl : baseUrl + "/");
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined && value !== "") {
        url.searchParams.set(key, String(value));
      }
    }
  }
  const response = await fetchImpl(url.toString(), init);
  const payload = await response.json();
  if (!response.ok) {
    const message = payload?.error?.message ?? response.statusText;
    throw new Error(message);
  }
  return payload as T;
}

export function createClient(config: ClientConfig = {}) {
  const baseUrl = config.baseUrl ?? (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080");
  return {
  async listArtists(params: listArtistsParams = {}, init?: RequestInit): Promise<{ "data"?: Artist[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: Artist[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/artists`, { "q": params["q"], "mbid": params["mbid"], "wikiquote_title": params["wikiquote_title"], "tag": params["tag"], "limit": params["limit"], "offset": params["offset"] }, init);
  },
  async getArtist(params: getArtistParams, init?: RequestInit): Promise<{ "data"?: Artist; }> {
    return request<{ "data"?: Artist; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/artists/${encodeURIComponent(String(params["artist_id"]))}`, undefined, init);
  },
  async getArtistQuotes(params: getArtistQuotesParams, init?: RequestInit): Promise<{ "data"?: Quote[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: Quote[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/artists/${encodeURIComponent(String(params["artist_id"]))}/quotes`, { "q": params["q"], "tag": params["tag"], "source": params["source"], "provenance_status": params["provenance_status"], "freshness_status": params["freshness_status"], "limit": params["limit"], "offset": params["offset"] }, init);
  },
  async getArtistRelated(params: getArtistRelatedParams, init?: RequestInit): Promise<{ "data"?: RelatedArtist[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: RelatedArtist[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/artists/${encodeURIComponent(String(params["artist_id"]))}/related`, undefined, init);
  },
  async getArtistReleases(params: getArtistReleasesParams, init?: RequestInit): Promise<{ "data"?: Release[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: Release[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/artists/${encodeURIComponent(String(params["artist_id"]))}/releases`, undefined, init);
  },
  async getArtistSetlists(params: getArtistSetlistsParams, init?: RequestInit): Promise<{ "data"?: Setlist[]; "meta"?: Record<string, unknown>; }> {
    return request<{ "data"?: Setlist[]; "meta"?: Record<string, unknown>; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/artists/${encodeURIComponent(String(params["artist_id"]))}/setlists`, undefined, init);
  },
  async listQuotes(params: listQuotesParams = {}, init?: RequestInit): Promise<{ "data"?: Quote[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: Quote[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/quotes`, { "artist": params["artist"], "artist_id": params["artist_id"], "q": params["q"], "tag": params["tag"], "source": params["source"], "provenance_status": params["provenance_status"], "freshness_status": params["freshness_status"], "sort": params["sort"], "limit": params["limit"], "offset": params["offset"] }, init);
  },
  async getRandomQuote(params: getRandomQuoteParams = {}, init?: RequestInit): Promise<{ "data"?: Quote; }> {
    return request<{ "data"?: Quote; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/quotes/random`, { "artist": params["artist"], "artist_id": params["artist_id"], "q": params["q"], "tag": params["tag"], "source": params["source"], "provenance_status": params["provenance_status"] }, init);
  },
  async getQuote(params: getQuoteParams, init?: RequestInit): Promise<{ "data"?: Quote; }> {
    return request<{ "data"?: Quote; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/quotes/${encodeURIComponent(String(params["quote_id"]))}`, undefined, init);
  },
  async getQuoteProvenance(params: getQuoteProvenanceParams, init?: RequestInit): Promise<{ "data"?: QuoteProvenance; }> {
    return request<{ "data"?: QuoteProvenance; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/quotes/${encodeURIComponent(String(params["quote_id"]))}/provenance`, undefined, init);
  },
  async getSource(params: getSourceParams, init?: RequestInit): Promise<{ "data"?: Source; }> {
    return request<{ "data"?: Source; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/sources/${encodeURIComponent(String(params["source_id"]))}`, undefined, init);
  },
  async listProviders(init?: RequestInit): Promise<{ "data"?: ProviderSummary[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: ProviderSummary[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, "/v1/providers", undefined, init);
  },
  async listProviderRuns(params: listProviderRunsParams, init?: RequestInit): Promise<{ "data"?: ProviderRun[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: ProviderRun[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/providers/${encodeURIComponent(String(params["provider"]))}/runs`, { "limit": params["limit"] }, init);
  },
  async listProviderErrors(params: listProviderErrorsParams, init?: RequestInit): Promise<{ "data"?: ProviderError[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: ProviderError[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/providers/${encodeURIComponent(String(params["provider"]))}/errors`, { "limit": params["limit"] }, init);
  },
  async listJobs(params: listJobsParams = {}, init?: RequestInit): Promise<{ "data"?: JobRun[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: JobRun[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/jobs`, { "limit": params["limit"] }, init);
  },
  async getJob(params: getJobParams, init?: RequestInit): Promise<{ "data"?: JobRun; }> {
    return request<{ "data"?: JobRun; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/jobs/${encodeURIComponent(String(params["job_id"]))}`, undefined, init);
  },
  async listReviewQueue(params: listReviewQueueParams = {}, init?: RequestInit): Promise<{ "data"?: ReviewQueueItem[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: ReviewQueueItem[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/review/queue`, { "provenance_status": params["provenance_status"], "limit": params["limit"], "offset": params["offset"] }, init);
  },
  async listStaleQuotes(params: listStaleQuotesParams = {}, init?: RequestInit): Promise<{ "data"?: Quote[]; "meta"?: ListMeta; "pagination"?: Pagination; }> {
    return request<{ "data"?: Quote[]; "meta"?: ListMeta; "pagination"?: Pagination; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/review/stale`, { "limit": params["limit"], "offset": params["offset"] }, init);
  },
  async searchCatalog(params: searchCatalogParams, init?: RequestInit): Promise<{ "data"?: SearchResults; "meta"?: ListMeta; }> {
    return request<{ "data"?: SearchResults; "meta"?: ListMeta; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/search`, { "q": params["q"] }, init);
  },
  async getStats(init?: RequestInit): Promise<{ "data"?: Record<string, unknown>; }> {
    return request<{ "data"?: Record<string, unknown>; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, "/v1/stats", undefined, init);
  },
  async getIntegrity(init?: RequestInit): Promise<{ "data"?: IntegrityReport; }> {
    return request<{ "data"?: IntegrityReport; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, "/v1/integrity", undefined, init);
  },
  async getLyrics(params: getLyricsParams, init?: RequestInit): Promise<{ "data"?: LyricsResult; "meta"?: Record<string, unknown>; }> {
    return request<{ "data"?: LyricsResult; "meta"?: Record<string, unknown>; }>(config.fetchImpl ?? globalThis.fetch.bind(globalThis), baseUrl, `/v1/lyrics`, { "artist": params["artist"], "track": params["track"], "provider": params["provider"] }, init);
  },
  };
}

export const apiClient = createClient();
