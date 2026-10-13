# Release Notes

## v1.1.27
GH#845 — republish with American English (en-US) content, completing the source-only GH#805 flip that never reached the Hub. Copy only — no functional or behaviour change.

## v1.1.26
GH#745 — declare per-step `output: {name, type}` on every execution step (status_data/text, stakeholder_update/text, polished_report/text, visual_spec/text, consistency_verdict/decision). Lights up the #744 rich flow-map. Content-only; no bindings or logic changes.

## v1.1.25
GH#645 Row 3b — migrate to K-037 dep-referenced schema. Strip 9 inline shared-content files and declare 9 hub-shared deps (UUID id + slug name + version + checksum from `gen-dep-checksums.mjs`). Closes pre-Step-3 inline-vendoring for this bundle.

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
