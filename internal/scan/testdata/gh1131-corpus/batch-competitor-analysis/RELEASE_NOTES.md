# Release Notes

## v1.1.26
GH#845 — republish with American English (en-US) content, completing the source-only GH#805 flip that never reached the Hub. Copy only — no functional or behaviour change.

## v1.1.25
GH#745 — declare per-step `output: {name, type}` on every execution step (competitor_profile/text, comparison_matrix/text, recommendations/text, polished_analysis/text). Lights up the #744 rich flow-map. Content-only; no bindings or logic changes.

## v1.1.24
Fix-forward after Row 3b v1.1.23 publish failure. The v1.1.23 per-skrpt CI's "Register version with Hub API" step failed because the consumer's source `manifest.id` (489ddd5d…) did not match the D1 catalog row's id (a67b75b6…) — a legacy drift from before Action 6 (`0bcc5ae0`) made publish-skrpt.mjs Step 2 INSERT use `manifest.id` for the D1 id column. v1.1.24 reconciles the source `manifest.id` to the catalog authoritative value (Row-5-equivalent for consumers) and republishes. Per Adj-1: no re-tag of v1.1.23; the orphaned GitHub release artefact stays inert (no D1 versions row, no consumer pinned it).

## v1.1.23
GH#645 Row 3b — migrate to K-037 dep-referenced schema. Strip 6 inline shared-content files and declare 6 hub-shared deps (UUID id + slug name + version + checksum from `gen-dep-checksums.mjs`). Internal slug references rewritten for E2 rename/mirror-drop pair(s): recommend-strategy→recommend-competitive-strategy. Closes pre-Step-3 inline-vendoring for this bundle.

## v1.1.22
Wave 2: re-signed with canonical engine signing pipeline.

## v1.1.21
Tags migrated inline into manifest (GH#586). tags.yaml retired.

## v1.1.20
Bundle re-signed with canonical engine signing pipeline (Wave 2 migration).

## v1.1.19
Signature fix — RELEASE_NOTES.md now included in integrity checksum.

## v1.1.18
Initial catalog release with full structural and content-quality validation. All scanner checks pass.
