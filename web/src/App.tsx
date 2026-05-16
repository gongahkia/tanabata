import { useDeferredValue, useEffect, useState } from "react";
import { Link, NavLink, Route, Routes, useParams } from "react-router-dom";

import {
  apiClient,
  type Artist,
  type JobRun,
  type ProviderSummary,
  type Quote,
  type QuoteProvenance,
  type RelatedArtist,
  type Release,
  type SearchResults
} from "./generated/client";

type LoadState<T> = {
  loading: boolean;
  error: string;
  data: T;
};

type ProvenanceFilter = "" | "verified" | "source_attributed" | "provider_attributed" | "ambiguous" | "needs_review";
type FreshnessFilter = "" | "fresh" | "aging" | "stale" | "unknown";

function App() {
  return (
    <div className="shell">
      <header className="hero">
        <div>
          <p className="eyebrow">Tanabata V2</p>
          <h1>Music knowledge, quote provenance, and pipeline history in one read-only catalog.</h1>
          <p className="lede">
            A portfolio-grade backend product for search, attribution, provider health, and ingestion visibility.
          </p>
        </div>
        <nav className="nav">
          <NavLink to="/">Discovery</NavLink>
          <NavLink to="/system">System</NavLink>
        </nav>
      </header>

      <main className="content">
        <Routes>
          <Route path="/" element={<DiscoveryPage />} />
          <Route path="/artists/:artistId" element={<ArtistPage />} />
          <Route path="/quotes/:quoteId" element={<QuotePage />} />
          <Route path="/system" element={<SystemPage />} />
        </Routes>
      </main>
    </div>
  );
}

function DiscoveryPage() {
  const [query, setQuery] = useState("frank");
  const [provenanceFilter, setProvenanceFilter] = useState<ProvenanceFilter>("");
  const [freshnessFilter, setFreshnessFilter] = useState<FreshnessFilter>("");
  const [sourceFilter, setSourceFilter] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [state, setState] = useState<LoadState<SearchResults>>({
    loading: true,
    error: "",
    data: { artists: [], quotes: [] }
  });

  useEffect(() => {
    const nextQuery = deferredQuery.trim();
    if (!nextQuery) {
      setState({ loading: false, error: "", data: { artists: [], quotes: [] } });
      return;
    }
    let active = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    const hasQuoteFilters = provenanceFilter || freshnessFilter || sourceFilter;
    const searchRequest = apiClient.searchCatalog({ q: nextQuery });
    const quoteRequest = hasQuoteFilters
      ? apiClient.listQuotes({
          q: nextQuery,
          provenance_status: provenanceFilter || undefined,
          freshness_status: freshnessFilter || undefined,
          source: sourceFilter || undefined,
          limit: 20
        })
      : null;
    Promise.all([searchRequest, quoteRequest])
      .then((response) => {
        if (!active) {
          return;
        }
        const [searchResponse, quoteResponse] = response;
        setState({
          loading: false,
          error: "",
          data: {
            artists: searchResponse.data?.artists ?? [],
            quotes: quoteResponse ? quoteResponse.data ?? [] : searchResponse.data?.quotes ?? []
          }
        });
      })
      .catch((error: Error) => {
        if (!active) {
          return;
        }
        setState({ loading: false, error: error.message, data: { artists: [], quotes: [] } });
      });
    return () => {
      active = false;
    };
  }, [deferredQuery, freshnessFilter, provenanceFilter, sourceFilter]);

  return (
    <section className="panel stack">
      <div className="section-head">
        <div>
          <p className="eyebrow">Discovery</p>
          <h2>Search artists and quotes through the FTS-backed catalog.</h2>
        </div>
        <label className="search">
          <span>Search</span>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Frank Ocean, hope, ambition..." />
        </label>
      </div>
      <div className="filters" aria-label="Discovery filters">
        <label>
          <span>Provenance</span>
          <select value={provenanceFilter} onChange={(event) => setProvenanceFilter(event.target.value as ProvenanceFilter)}>
            <option value="">Any provenance</option>
            <option value="verified">Verified</option>
            <option value="source_attributed">Source attributed</option>
            <option value="provider_attributed">Provider attributed</option>
            <option value="ambiguous">Ambiguous</option>
            <option value="needs_review">Needs review</option>
          </select>
        </label>
        <label>
          <span>Freshness</span>
          <select value={freshnessFilter} onChange={(event) => setFreshnessFilter(event.target.value as FreshnessFilter)}>
            <option value="">Any freshness</option>
            <option value="fresh">Fresh</option>
            <option value="aging">Aging</option>
            <option value="stale">Stale</option>
            <option value="unknown">Unknown</option>
          </select>
        </label>
        <label>
          <span>Source</span>
          <select value={sourceFilter} onChange={(event) => setSourceFilter(event.target.value)}>
            <option value="">Any source</option>
            <option value="wikiquote">Wikiquote</option>
            <option value="tanabata_curated">Curated</option>
            <option value="quotefancy">QuoteFancy</option>
          </select>
        </label>
      </div>

      {state.error ? <ErrorState title="Search failed" message={state.error} /> : null}

      <div className="grid">
        <section className="card">
          <h3>Artists</h3>
          {state.loading ? <LoadingState label="Loading artists" /> : null}
          {!state.loading && !state.error && (state.data.artists ?? []).length === 0 ? (
            <EmptyState title="No artists found" message="Try a broader artist name, alias, or genre-linked phrase." />
          ) : null}
          <ul className="list">
            {(state.data.artists ?? []).map((artist) => (
              <li key={artist.artist_id}>
                <Link to={`/artists/${artist.artist_id}`}>{artist.name}</Link>
                <span>{artist.country || "Unknown region"}</span>
              </li>
            ))}
          </ul>
        </section>

        <section className="card">
          <h3>Quotes</h3>
          {state.loading ? <LoadingState label="Loading quotes" /> : null}
          {!state.loading && !state.error && (state.data.quotes ?? []).length === 0 ? (
            <EmptyState title="No quotes found" message="Search relevance is strict; try fewer terms or a known artist." />
          ) : null}
          <ul className="list">
            {(state.data.quotes ?? []).map((quote) => (
              <li key={quote.quote_id}>
                <Link to={`/quotes/${quote.quote_id}`}>{quote.text}</Link>
                <span>{quote.artist_name}</span>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </section>
  );
}

function ArtistPage() {
  const { artistId = "" } = useParams();
  const [artist, setArtist] = useState<LoadState<Artist | null>>({ loading: true, error: "", data: null });
  const [quotes, setQuotes] = useState<Quote[]>([]);
  const [releases, setReleases] = useState<Release[]>([]);
  const [related, setRelated] = useState<RelatedArtist[]>([]);

  useEffect(() => {
    let active = true;
    setArtist({ loading: true, error: "", data: null });
    Promise.all([
      apiClient.getArtist({ artist_id: artistId }),
      apiClient.getArtistQuotes({ artist_id: artistId, limit: 8 }),
      apiClient.getArtistReleases({ artist_id: artistId }),
      apiClient.getArtistRelated({ artist_id: artistId })
    ])
      .then(([artistResponse, quotesResponse, releasesResponse, relatedResponse]) => {
        if (!active) {
          return;
        }
        setArtist({ loading: false, error: "", data: artistResponse.data ?? null });
        setQuotes(quotesResponse.data ?? []);
        setReleases(releasesResponse.data ?? []);
        setRelated(relatedResponse.data ?? []);
      })
      .catch((error: Error) => {
        if (!active) {
          return;
        }
        setArtist({ loading: false, error: error.message, data: null });
      });
    return () => {
      active = false;
    };
  }, [artistId]);

  return (
    <section className="panel stack">
      <Link className="back" to="/">
        Back to discovery
      </Link>
      {artist.loading ? <LoadingState label="Loading artist" /> : null}
      {artist.error ? <ErrorState title="Artist failed to load" message={artist.error} /> : null}
      {!artist.loading && !artist.error && !artist.data ? <EmptyState title="Artist not found" message="This catalog entry is not available." /> : null}
      {artist.data ? (
        <>
          <div className="section-head">
            <div>
              <p className="eyebrow">Artist Detail</p>
              <h2>{artist.data.name}</h2>
              <p className="lede">{artist.data.bio_summary || artist.data.description || "No enriched biography yet."}</p>
            </div>
            <dl className="facts">
              <div>
                <dt>Genres</dt>
                <dd>{(artist.data.genres ?? []).join(", ") || "Unclassified"}</dd>
              </div>
              <div>
                <dt>Providers</dt>
                <dd>{Object.keys(artist.data.provider_status ?? {}).join(", ") || "Legacy"}</dd>
              </div>
            </dl>
          </div>

          <div className="grid">
            <section className="card">
              <h3>Quotes</h3>
              {quotes.length === 0 ? <EmptyState title="No quotes yet" message="This artist has no attributed quotes in the current catalog." /> : null}
              <ul className="list">
                {quotes.map((quote) => (
                  <li key={quote.quote_id}>
                    <Link to={`/quotes/${quote.quote_id}`}>{quote.text}</Link>
                    <span>{quote.provenance_status}</span>
                  </li>
                ))}
              </ul>
            </section>
            <section className="card">
              <h3>Releases</h3>
              {releases.length === 0 ? <EmptyState title="No releases yet" message="MusicBrainz enrichment has not added releases for this artist." /> : null}
              <ul className="list">
                {releases.map((release) => (
                  <li key={release.release_id}>
                    <span>{release.title}</span>
                    <span>{release.year ?? "Unknown year"}</span>
                  </li>
                ))}
              </ul>
            </section>
            <section className="card">
              <h3>Related</h3>
              {related.length === 0 ? <EmptyState title="No related artists yet" message="Last.fm similarity data is not available for this artist." /> : null}
              <ul className="list">
                {related.map((item) => (
                  <li key={`${item.artist_id}-${item.provider}`}>
                    <Link to={`/artists/${item.artist_id}`}>{item.name}</Link>
                    <span>{item.provider}</span>
                  </li>
                ))}
              </ul>
            </section>
          </div>
        </>
      ) : null}
    </section>
  );
}

function QuotePage() {
  const { quoteId = "" } = useParams();
  const [quote, setQuote] = useState<LoadState<Quote | null>>({ loading: true, error: "", data: null });
  const [provenance, setProvenance] = useState<QuoteProvenance | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([apiClient.getQuote({ quote_id: quoteId }), apiClient.getQuoteProvenance({ quote_id: quoteId })])
      .then(([quoteResponse, provenanceResponse]) => {
        if (!active) {
          return;
        }
        setQuote({ loading: false, error: "", data: quoteResponse.data ?? null });
        setProvenance(provenanceResponse.data ?? null);
      })
      .catch((error: Error) => {
        if (!active) {
          return;
        }
        setQuote({ loading: false, error: error.message, data: null });
      });
    return () => {
      active = false;
    };
  }, [quoteId]);

  return (
    <section className="panel stack">
      <Link className="back" to="/">
        Back to discovery
      </Link>
      {quote.loading ? <LoadingState label="Loading quote" /> : null}
      {quote.error ? <ErrorState title="Quote failed to load" message={quote.error} /> : null}
      {!quote.loading && !quote.error && !quote.data ? <EmptyState title="Quote not found" message="This quote ID is not present in the catalog." /> : null}
      {quote.data ? (
        <>
          <blockquote className="quote-card">
            <p>{quote.data.text}</p>
            <footer>
              <Link to={`/artists/${quote.data.artist_id}`}>{quote.data.artist_name}</Link>
            </footer>
          </blockquote>
          {provenance ? (
            <section className="card">
              <h3>Provenance</h3>
              <dl className="facts">
                <div>
                  <dt>Status</dt>
                  <dd>{provenance.provenance_status}</dd>
                </div>
                <div>
                  <dt>Confidence</dt>
                  <dd>{(provenance.confidence_score ?? 0).toFixed(2)}</dd>
                </div>
                <div>
                  <dt>Origin</dt>
                  <dd>{provenance.provider_origin}</dd>
                </div>
              </dl>
              <ul className="list evidence">
                {(provenance.evidence ?? []).map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
              {provenance.source?.url ? (
                <a className="external" href={provenance.source.url} target="_blank" rel="noreferrer">
                  Inspect source
                </a>
              ) : null}
              <ProvenanceComparison quote={quote.data} provenance={provenance} />
            </section>
          ) : null}
        </>
      ) : null}
    </section>
  );
}

function ProvenanceComparison({ quote, provenance }: { quote: Quote; provenance: QuoteProvenance }) {
  const rows = [
    {
      label: "Verification",
      primary: provenance.provenance_status,
      comparison: quote.freshness_status ? `${quote.freshness_status}: ${quote.freshness_reason ?? "freshness policy"}` : "freshness unavailable"
    },
    {
      label: "Source",
      primary: provenance.source?.title || provenance.source?.url || quote.source_type || "No source",
      comparison: provenance.source?.provider || provenance.provider_origin || "No provider"
    },
    {
      label: "Evidence",
      primary: `${(provenance.evidence ?? []).length} evidence items`,
      comparison: `confidence ${(provenance.confidence_score ?? 0).toFixed(2)}`
    }
  ];

  return (
    <div className="comparison-panel">
      <div>
        <p className="eyebrow">Source Comparison</p>
        <h4>Why this quote is trusted or queued</h4>
      </div>
      <ul>
        {rows.map((row) => (
          <li key={row.label}>
            <span>{row.label}</span>
            <strong>{row.primary}</strong>
            <em>{row.comparison}</em>
          </li>
        ))}
      </ul>
    </div>
  );
}

function SystemPage() {
  const [providers, setProviders] = useState<ProviderSummary[]>([]);
  const [jobs, setJobs] = useState<JobRun[]>([]);
  const [stats, setStats] = useState<Record<string, unknown>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    Promise.all([apiClient.listProviders(), apiClient.listJobs({ limit: 10 }), apiClient.getStats()])
      .then(([providersResponse, jobsResponse, statsResponse]) => {
        if (!active) {
          return;
        }
        setProviders(providersResponse.data ?? []);
        setJobs(jobsResponse.data ?? []);
        setStats((statsResponse.data as Record<string, unknown>) ?? {});
      })
      .catch((loadError: Error) => {
        if (!active) {
          return;
        }
        setError(loadError.message);
      });
    return () => {
      active = false;
    };
  }, []);

  return (
    <section className="panel stack">
      <div className="section-head">
        <div>
          <p className="eyebrow">System</p>
          <h2>Provider health, freshness, and ingestion history.</h2>
        </div>
      </div>
      {error ? <ErrorState title="System state failed to load" message={error} /> : null}
      <div className="grid">
        <section className="card">
          <h3>Catalog Stats</h3>
          {!error && Object.keys(stats).length === 0 ? <EmptyState title="No stats reported" message="The API returned an empty stats payload." /> : null}
          <ul className="list">
            {Object.entries(stats).map(([key, value]) => (
              <li key={key}>
                <span>{key.split("_").join(" ")}</span>
                <strong>{String(value)}</strong>
              </li>
            ))}
          </ul>
        </section>
        <section className="card">
          <h3>Providers</h3>
          {!error && providers.length === 0 ? <EmptyState title="No providers configured" message="Provider inventory is empty for this environment." /> : null}
          <ul className="list">
            {providers.map((provider) => (
              <li key={provider.provider}>
                <span>{provider.provider}</span>
                <span>{provider.enabled ? "enabled" : "disabled"}</span>
              </li>
            ))}
          </ul>
        </section>
        <section className="card">
          <h3>Recent Jobs</h3>
          {!error && jobs.length === 0 ? <EmptyState title="No ingestion jobs yet" message="Run the ingestion CLI to populate job history." /> : null}
          <ul className="list">
            {jobs.map((job) => (
              <li key={job.job_id}>
                <span>{job.name}</span>
                <span>{job.status}</span>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </section>
  );
}

function LoadingState({ label }: { label: string }) {
  return (
    <div className="state state-loading" role="status" aria-live="polite">
      <span className="spinner" aria-hidden="true" />
      <span>{label}...</span>
    </div>
  );
}

function EmptyState({ title, message }: { title: string; message: string }) {
  return (
    <div className="state state-empty">
      <strong>{title}</strong>
      <span>{message}</span>
    </div>
  );
}

function ErrorState({ title, message }: { title: string; message: string }) {
  return (
    <div className="state state-error" role="alert">
      <strong>{title}</strong>
      <span>{message}</span>
    </div>
  );
}

export default App;
