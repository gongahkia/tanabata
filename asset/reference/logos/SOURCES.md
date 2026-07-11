# Diagram logo sources

These PNGs are rendering assets for `../diagram.py`. They have transparent backgrounds and are resized to a 256 px maximum dimension.

| Asset(s) | Source |
| --- | --- |
| `docker`, `github`, `github-actions`, `go`, `lastfm`, `musicbrainz`, `openapi`, `opentelemetry`, `prometheus`, `python`, `render`, `sqlite`, `wikidata`, `wikiquote` | [Simple Icons CDN](https://cdn.simpleicons.org/), using its documented brand-color SVG endpoint; rasterized locally to PNG. Simple Icons is [CC0-1.0](https://github.com/simple-icons/simple-icons/blob/develop/LICENSE.md). |
| `gin` | [gin-gonic/logo](https://github.com/gin-gonic/logo/blob/master/color.png), rasterized locally to PNG. |
| `lrclib` | [LRCLIB favicon](https://lrclib.net/favicon.ico), rasterized locally to PNG. |
| `setlistfm` | [setlist.fm favicon](https://www.setlist.fm/favicon.ico), rasterized locally to PNG. |
| `tanabata` | Project-owned [`asset/logo/tanabata.webp`](../../logo/tanabata.webp), rasterized locally to PNG. |
| `json` | Locally drawn generic JSON-artifact icon. |

The source downloads were used only to create these rasterized diagram assets; the diagram does not fetch remote assets at render time.
