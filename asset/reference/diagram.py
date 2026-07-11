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
            "CI workflow on push/PR\ntests, coverage, lint,\ncontainer + endpoint smoke,\nvulnerability scans",
            "github-actions.png",
        )
        api_docs = service(
            "API docs workflow\nbuild OpenAPI docs\nand deploy GitHub Pages",
            "github-actions.png",
        )
        github_pages = service(
            "GitHub Pages\nrendered API docs",
            "github.png",
        )

        repository >> Edge(label=".github/workflows/scrape.yml") >> catalog_refresh
        repository >> Edge(label=".github/workflows/ci.yml") >> ci
        repository >> Edge(label=".github/workflows/docs.yml") >> api_docs
        api_docs >> Edge(label="publishes") >> github_pages

    with Cluster("Monthly catalog refresh pipeline"):
        wikiquote_source = service(
            "Wikiquote\nMediaWiki API\nquote source",
            "wikiquote.png",
        )
        scraper = service(
            "Python scraper\nscraper/main.py\nstdlib HTTP + HTMLParser",
            "python.png",
        )
        quotes_json = service(
            "api/data/quotes.json\nWikiquote refresh artifact",
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

        wikiquote_source >> Edge(label="queried monthly") >> scraper
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
            "Render web service\ntanabata.onrender.com\npublic /v1 API",
            "render.png",
        )
        runtime_catalog = service(
            "catalog.sqlite\nimage-seeded catalog +\noptional persisted volume\nread path + provider cache",
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
            "REST API surface\n/livez /readyz /health /metrics\nlegacy /quotes routes\n/v1 catalog, search, providers,\njobs, review, integrity, docs\n/v1 lyrics, setlists, webhooks",
            "openapi.png",
        )

        refreshed_artifacts >> Edge(label="included at image build") >> docker
        docker >> Edge(label="runs container") >> api_server
        docker >> Edge(label="seeds / mounts catalog") >> runtime_catalog
        docker >> Edge(label="deployed public service") >> render
        runtime_catalog >> Edge(label="CATALOG_PATH") >> api_server
        api_server >> Edge(label="registers routes") >> gin_router
        openapi_contract >> Edge(label="contract tests + optional middleware") >> gin_router
        gin_router >> Edge(label="serves JSON envelopes") >> api_surface

    with Cluster("Runtime provider lookups"):
        lyrics = service("LRCLIB\nlyrics.ovh fallback", "lrclib.png")
        setlistfm = service("setlist.fm\nlive setlists", "setlistfm.png")
        webhook_targets = service(
            "Webhook subscribers\nsigned event POSTs\nretry + disable on failure",
            "gin.png",
        )
        provider_cache = service(
            "provider/cache tables\ninside catalog.sqlite\ncached lookups, provider\nruns, errors, subscriptions",
            "sqlite.png",
        )

        api_surface >> Edge(label="on-demand HTTP lookups") >> [lyrics, setlistfm]
        [lyrics, setlistfm] >> Edge(label="cache hits/writes\nand failures") >> provider_cache
        provider_cache >> Edge(label="subscriptions + delivery state") >> api_surface
        api_surface >> Edge(label="signed webhook delivery") >> webhook_targets

    with Cluster("Observability"):
        prometheus = service(
            "Prometheus metrics\n/metrics\nHTTP + provider counters",
            "prometheus.png",
        )
        otel = service(
            "OpenTelemetry traces\nOTLP when configured;\nstdout in dev, else noop\n10% parent-based default",
            "opentelemetry.png",
        )

        gin_router >> Edge(label="metrics middleware") >> prometheus
        gin_router >> Edge(label="request spans") >> otel

    consumers = Users("API consumers")

    api_surface >> Edge(label="HTTP API responses") >> consumers
    catalog_refresh >> Edge(label="runs scraper and ingest") >> wikiquote_source
