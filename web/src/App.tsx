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
    apiClient
      .searchCatalog({ q: nextQuery })
      .then((response) => {
        if (!active) {
          return;
        }
        setState({ loading: false, error: "", data: response.data ?? { artists: [], quotes: [] } });
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
  }, [deferredQuery]);

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

      {state.error ? <p className="error">{state.error}</p> : null}

      <div className="grid">
        <section className="card">
          <h3>Artists</h3>
          {state.loading ? <p>Loading artists...</p> : null}
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
          {state.loading ? <p>Loading quotes...</p> : null}
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
      {artist.loading ? <p>Loading artist...</p> : null}
      {artist.error ? <p className="error">{artist.error}</p> : null}
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
      {quote.loading ? <p>Loading quote...</p> : null}
      {quote.error ? <p className="error">{quote.error}</p> : null}
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
            </section>
          ) : null}
        </>
      ) : null}
    </section>
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
      {error ? <p className="error">{error}</p> : null}
      <div className="grid">
        <section className="card">
          <h3>Catalog Stats</h3>
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

export default App;
