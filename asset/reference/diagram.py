from pathlib import Path

from diagrams import Cluster, Diagram, Edge
from diagrams.custom import Custom
from diagrams.onprem.client import Users


ROOT = Path(__file__).resolve().parents[2]
REFERENCE_DIR = ROOT / "asset" / "reference"
LOGO_DIR = REFERENCE_DIR / "logos"

REQUIRED_LOGOS = [
    "docker.png",
    "gin.png",
    "github.png",
    "github-actions.png",
    "go.png",
    "json.png",
    "lastfm.png",
    "lrclib.png",
    "musicbrainz.png",
    "openapi.png",
    "opentelemetry.png",
    "prometheus.png",
    "python.png",
    "quotefancy.png",
    "render.png",
    "setlistfm.png",
    "sqlite.png",
    "tanabata.png",
    "wikidata.png",
    "wikiquote.png",
]

missing_logos = [name for name in REQUIRED_LOGOS if not (LOGO_DIR / name).exists()]
if missing_logos:
    raise FileNotFoundError(
        "Missing logo PNGs in asset/reference/logos: " + ", ".join(missing_logos)
    )


def logo(name: str) -> str:
    return str(LOGO_DIR / name)


def service(label: str, icon: str) -> Custom:
    return Custom(label, logo(icon))


graph_attr = {
    "bgcolor": "white",
    "concentrate": "false",
    "fontname": "Helvetica",
    "fontsize": "26",
    "labelloc": "t",
    "nodesep": "0.56",
    "pad": "0.65",
    "ranksep": "0.92",
    "splines": "spline",
}

node_attr = {
    "fontname": "Helvetica",
    "fontsize": "13",
}

edge_attr = {
    "color": "#6b7785",
    "fontcolor": "#4b5563",
    "fontname": "Helvetica",
    "fontsize": "11",
    "penwidth": "1.5",
}


with Diagram(
    "Tanabata REST API Architecture",
    direction="TB",
    filename=str(REFERENCE_DIR / "architecture"),
    outformat="png",
    show=False,
    graph_attr=graph_attr,
    node_attr=node_attr,
    edge_attr=edge_attr,
):
    with Cluster("GitHub automation"):
        repository = service(
            "GitHub repository\nsource, workflows,\nversioned data artifacts",
            "github.png",
        )
        catalog_refresh = service(
            "Catalog refresh workflow\nschedule: 06:00 UTC\n1st of every month\n+ manual dispatch",
            "github-actions.png",
        )
        ci = service(
            "CI workflow on push/PR\ngo test, coverage,\ngolangci-lint,\ncontainer smoke",
            "github-actions.png",
        )

        repository >> Edge(label=".github/workflows/scrape.yml") >> catalog_refresh
        repository >> Edge(label=".github/workflows/ci.yml") >> ci

    with Cluster("Monthly catalog refresh pipeline"):
        quotefancy = service("QuoteFancy\nscrape source", "quotefancy.png")
        scraper = service(
            "Python scraper\nscraper/main.py\nPlaywright Chromium",
            "python.png",
        )
        quotes_json = service(
            "api/data/quotes.json\nlegacy scrape artifact",
            "json.png",
        )
        curated_bundles = service(
            "Curated bundles\nquotes, works,\nsamples, performances,\nmisquotes",
            "tanabata.png",
        )
        ingest_cli = service(
            "Go ingestion CLI\napi/cmd/ingest\n-bootstrap -all",
            "go.png",
        )

        with Cluster("Ingest-time enrichment"):
            musicbrainz = service("MusicBrainz\nartist IDs,\nreleases", "musicbrainz.png")
            wikidata = service("Wikidata\nentities,\nlinks", "wikidata.png")
            wikiquote = service("Wikiquote\nquote pages", "wikiquote.png")
            lastfm = service("Last.fm\ntags,\nrelated artists", "lastfm.png")

        sqlite_catalog = service(
            "SQLite catalog\napi/data/catalog.sqlite\ncore tables + FTS5\njobs, snapshots,\naudit, provider cache",
            "sqlite.png",
        )
        refreshed_artifacts = service(
            "Committed refresh output\nquotes.json + catalog.sqlite",
            "github.png",
        )

        quotefancy >> Edge(label="scraped monthly") >> scraper
        scraper >> Edge(label="writes refreshed JSON") >> quotes_json
        [quotes_json, curated_bundles] >> Edge(
            label="bootstrap + curated import"
        ) >> ingest_cli
        [musicbrainz, wikidata, wikiquote, lastfm] >> Edge(
            label="HTTP adapters\nretry, timeout,\nrate-limit, cooldown"
        ) >> ingest_cli
        ingest_cli >> Edge(
            label="migrate, seed,\nrefresh search indexes,\nrecord lineage"
        ) >> sqlite_catalog
        sqlite_catalog >> Edge(label="committed by workflow") >> refreshed_artifacts

    with Cluster("Runtime API service"):
        docker = service(
            "Docker image / compose\napi service\n18080 -> 8080",
            "docker.png",
        )
        render = service(
            "Render target\ncurrent deployment inactive\nas of 2026-06-01",
            "render.png",
        )
        runtime_catalog = service(
            "catalog.sqlite mount\nsame versioned SQLite file\nread path + runtime cache",
            "sqlite.png",
        )
        api_server = service(
            "Go API process\napi/main.go\nread-only catalog startup",
            "go.png",
        )
        gin_router = service(
            "Gin HTTP router\nrequest ID, CORS,\nstructured logs,\nrecovery middleware",
            "gin.png",
        )
        openapi_contract = service(
            "OpenAPI contract\nopenapi/openapi.json\noptional runtime validation",
            "openapi.png",
        )
        api_surface = service(
            "REST API surface\n/livez /readyz /health\nlegacy /quotes routes\n/v1 artists, quotes, works,\nrecordings, samples, claims\n/v1 search, providers, jobs,\ntimeline, review, integrity\n/v1 lyrics and setlists",
            "openapi.png",
        )

        docker >> Edge(label="runs container") >> api_server
        docker >> Edge(label="deploy target currently down", style="dashed") >> render
        refreshed_artifacts >> Edge(label="checked out by runtime") >> docker
        refreshed_artifacts >> Edge(label="mounts api/data/catalog.sqlite") >> runtime_catalog
        runtime_catalog >> Edge(label="CATALOG_PATH") >> api_server
        api_server >> Edge(label="registers routes") >> gin_router
        openapi_contract >> Edge(label="contract tests + optional middleware") >> gin_router
        gin_router >> Edge(label="serves JSON envelopes") >> api_surface

    with Cluster("Runtime provider lookups"):
        lrclib = service("LRCLIB\nsynced lyrics\nlyrics.ovh fallback", "lrclib.png")
        setlistfm = service("setlist.fm\nlive setlists", "setlistfm.png")
        provider_cache = service(
            "provider_cache tables\ninside catalog.sqlite\ncached lyrics/setlists\nprovider runs/errors",
            "sqlite.png",
        )

        api_surface >> Edge(label="on-demand HTTP lookups") >> [lrclib, setlistfm]
        [lrclib, setlistfm] >> Edge(label="cache hits/writes\nand failures") >> provider_cache

    with Cluster("Observability"):
        prometheus = service(
            "Prometheus metrics\n/metrics\nHTTP + provider counters",
            "prometheus.png",
        )
        otel = service(
            "OpenTelemetry traces\nstdout exporter\n20% parent-based sampling",
            "opentelemetry.png",
        )

        gin_router >> Edge(label="metrics middleware") >> prometheus
        gin_router >> Edge(label="request spans") >> otel

    consumers = Users("API consumers")

    api_surface >> Edge(label="HTTP GET responses") >> consumers
    catalog_refresh >> Edge(label="runs scraper and ingest") >> quotefancy
