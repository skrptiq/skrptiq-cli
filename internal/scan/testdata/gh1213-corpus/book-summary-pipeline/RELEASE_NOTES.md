# Release Notes

## v1.1.28
GH#845 — republish with American English (en-US) content, completing the source-only GH#805 flip that never reached the Hub. Copy only — no functional or behaviour change.

## v1.1.27
GH#745 — declare per-step `output: {name, type}` on every execution step (key_ideas/list, summary/text, citations/list, polished_summary/text). Lights up the #744 rich flow-map with named, typed outputs. Content-only; no bindings or logic changes.

## v1.1.26
Fix-forward after Row 3b v1.1.25 publish failure. The v1.1.25 per-skrpt CI's "Register version with Hub API" step failed because the consumer's source `manifest.id` (b700d0f2…) did not match the D1 catalog row's id (3afa2e9c…) — a legacy drift from before Action 6 (`0bcc5ae0`) made publish-skrpt.mjs Step 2 INSERT use `manifest.id` for the D1 id column. v1.1.26 reconciles the source `manifest.id` to the catalog authoritative value (Row-5-equivalent for consumers) and republishes. Per Adj-1: no re-tag of v1.1.25; the orphaned GitHub release artefact stays inert (no D1 versions row, no consumer pinned it).

## v1.1.25
GH#645 Row 3b — migrate to K-037 dep-referenced schema. Strip 12 inline shared-content files and declare 12 hub-shared deps (UUID id + slug name + version + checksum from `gen-dep-checksums.mjs`). Internal slug references rewritten for E2 rename/mirror-drop pair(s): summary-synthesis→synthesise-book-summary. Closes pre-Step-3 inline-vendoring for this bundle.

## v1.1.24
Wave 2: re-signed with canonical engine signing pipeline.

## v1.1.23
Tags migrated inline into manifest (GH#586). tags.yaml retired.

## v1.1.22
Bundle re-signed with canonical engine signing pipeline (Wave 2 migration).

## v1.1.21
Signature fix — RELEASE_NOTES.md now included in integrity checksum.

## v1.1.20
Initial catalog release with full structural and content-quality validation. All scanner checks pass.
