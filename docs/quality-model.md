# Quote Quality Model

Tanabata treats quote quality as an explainable catalog decision, not a single magic score.

## Inputs

- Provenance state: `verified`, `source_attributed`, `provider_attributed`, `ambiguous`, or `needs_review`.
- Confidence score: provider/importer confidence from `0.0` to `1.0`.
- Evidence: human-readable evidence rows attached to the quote.
- Source metadata: provider, URL, title, publisher, license, and retrieval timestamp.
- Freshness: computed from `last_verified_at` using the catalog policy.

## Provenance States

- `verified`: strongest state; source attribution and evidence agree.
- `source_attributed`: a concrete source exists, but no additional verification pass has promoted it.
- `provider_attributed`: provider supplied attribution, but source metadata is incomplete.
- `ambiguous`: evidence conflicts or multiple plausible attributions exist.
- `needs_review`: imported or merged data is not strong enough for display without review context.

## Freshness Policy

- `fresh`: verified in the last 90 days.
- `aging`: verified between 90 and 180 days ago.
- `stale`: verified more than 180 days ago.
- `unknown`: no usable `last_verified_at` timestamp.

Freshness is intentionally separate from confidence. A quote can be high-confidence but stale, which makes it a refresh candidate rather than a bad quote.

## Review Queue Rules

The review queue prioritizes weak provenance before ordinary freshness refreshes:

- `needs_review` first.
- `ambiguous` second.
- `provider_attributed` third.
- Lower confidence before higher confidence within each state.
- Older verification timestamps before newer ones.

Each review item includes a reason and risk score so the UI can explain why it appears.

## Merge Quality Rules

When duplicate or near-duplicate quotes are imported, Tanabata keeps the strongest record and merges supporting evidence:

- Higher provenance rank wins over lower rank.
- Higher confidence breaks provenance ties.
- Source-backed records win over source-less records.
- Longer evidence and richer source metadata are preserved.
- Text normalization and similarity prevent punctuation-only duplicates from fragmenting the catalog.

The result should be auditable: quote detail pages expose source metadata, evidence, freshness, and provider origin.
