import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import App from "./App";

const responseMap: Record<string, unknown> = {
  "/v1/search?q=frank": {
    data: {
      artists: [{ artist_id: "frank", name: "Frank Ocean", aliases: [], genres: [], links: [], provider_status: {}, life_span: {} }],
      quotes: [{ quote_id: "q1", text: "Work hard in silence.", artist_id: "frank", artist_name: "Frank Ocean", tags: [], provenance_status: "source_attributed", confidence_score: 0.9 }]
    }
  },
  "/v1/artists/frank": {
    data: {
      artist_id: "frank",
      name: "Frank Ocean",
      aliases: ["Frank Ocean"],
      genres: ["rnb"],
      links: [],
      provider_status: { wikiquote: "fetched" },
      life_span: {},
      bio_summary: "American singer-songwriter"
    }
  },
  "/v1/artists/frank/quotes?limit=8": {
    data: [{ quote_id: "q1", text: "Work hard in silence.", artist_id: "frank", artist_name: "Frank Ocean", tags: [], provenance_status: "source_attributed", confidence_score: 0.9 }]
  },
  "/v1/artists/frank/releases": {
    data: [{ release_id: "blonde", title: "Blonde", year: 2016, provider: "musicbrainz" }]
  },
  "/v1/artists/frank/related": {
    data: [{ artist_id: "sza", name: "SZA", relation: "similar", score: 0.5, provider: "lastfm" }]
  },
  "/v1/quotes/q1": {
    data: { quote_id: "q1", text: "Work hard in silence.", artist_id: "frank", artist_name: "Frank Ocean", tags: [], provenance_status: "source_attributed", confidence_score: 0.9 }
  },
  "/v1/quotes/q1/provenance": {
    data: {
      quote_id: "q1",
      provenance_status: "source_attributed",
      confidence_score: 0.9,
      provider_origin: "wikiquote",
      evidence: ["Matched Wikiquote page", "Source URL: https://example.com"],
      source: { url: "https://example.com" }
    }
  },
  "/v1/providers": { data: [] },
  "/v1/jobs?limit=10": { data: [] },
  "/v1/stats": { data: { artists: 1, quotes: 1 } }
};

beforeEach(() => {
  globalThis.fetch = vi.fn(async (input: string | URL) => {
    const url = new URL(String(input));
    const payload = responseMap[url.pathname + url.search] ?? { data: [] };
    return new Response(JSON.stringify(payload), { status: 200, headers: { "Content-Type": "application/json" } });
  }) as typeof fetch;
});

it("renders discovery results from the generated client", async () => {
  render(
    <MemoryRouter initialEntries={["/"]}>
      <App />
    </MemoryRouter>
  );

  await waitFor(() => expect(screen.getAllByText("Frank Ocean").length).toBeGreaterThan(0));
  expect(screen.getByText("Work hard in silence.")).toBeInTheDocument();
});

it("renders empty discovery states", async () => {
  globalThis.fetch = vi.fn(async () => {
    return new Response(JSON.stringify({ data: { artists: [], quotes: [] } }), { status: 200, headers: { "Content-Type": "application/json" } });
  }) as typeof fetch;

  render(
    <MemoryRouter initialEntries={["/"]}>
      <App />
    </MemoryRouter>
  );

  await waitFor(() => expect(screen.getByText("No artists found")).toBeInTheDocument());
  expect(screen.getByText("No quotes found")).toBeInTheDocument();
});

it("renders API error state", async () => {
  globalThis.fetch = vi.fn(async () => {
    return new Response(JSON.stringify({ error: { code: "search_failed", message: "upstream unavailable" } }), {
      status: 503,
      headers: { "Content-Type": "application/json" }
    });
  }) as typeof fetch;

  render(
    <MemoryRouter initialEntries={["/"]}>
      <App />
    </MemoryRouter>
  );

  await waitFor(() => expect(screen.getByText("Search failed")).toBeInTheDocument());
  expect(screen.getByText("upstream unavailable")).toBeInTheDocument();
});

it("renders system page", async () => {
  render(
    <MemoryRouter initialEntries={["/system"]}>
      <App />
    </MemoryRouter>
  );

  await waitFor(() => expect(screen.getByText("Catalog Stats")).toBeInTheDocument());
});

it("renders artist detail page", async () => {
  render(
    <MemoryRouter initialEntries={["/artists/frank"]}>
      <App />
    </MemoryRouter>
  );

  await waitFor(() => expect(screen.getByText("American singer-songwriter")).toBeInTheDocument());
  expect(screen.getByText("Blonde")).toBeInTheDocument();
  expect(screen.getByText("SZA")).toBeInTheDocument();
});

it("renders quote provenance page", async () => {
  render(
    <MemoryRouter initialEntries={["/quotes/q1"]}>
      <App />
    </MemoryRouter>
  );

  await waitFor(() => expect(screen.getByText("Provenance")).toBeInTheDocument());
  expect(screen.getByText("Matched Wikiquote page")).toBeInTheDocument();
  expect(screen.getByText("Inspect source")).toBeInTheDocument();
});
