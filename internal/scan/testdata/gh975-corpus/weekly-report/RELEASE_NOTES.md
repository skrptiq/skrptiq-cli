# Release Notes

## v1.0.15
GH#833 — drop the embedded `gmail-mcp` + `google-calendar-mcp` service nodes; dep-reference the canonicals. contents.services 2→0.

## v1.0.14
GH#745 — declare per-step `output: {name, type}` on every execution step (week_data/text, activity_analysis/text, weekly_report/text, polished_report/text). Lights up the #744 rich flow-map. Content-only. (Completes the #745 sweep for this skrpt after the #754 error fix.)

## v1.0.13
GH#754 — restore the driving prompt (`synthesise-report`) dropped by the GH#645 K-037 migration and repoint the report step to it (was referencing the skill slug `synthesise-weekly-report` → `execution_prompt_wrong_type` ERROR). Reconcile the restored prompt’s connection target. Canonical scan clean.

## v1.0.12
Fix-forward after Row 3b v1.0.11 publish failure. The v1.0.11 per-skrpt CI's "Register version with Hub API" step failed because the consumer's source `manifest.id` (42e87e6d…) did not match the D1 catalog row's id (4b68a002…) — a legacy drift from before Action 6 (`0bcc5ae0`) made publish-skrpt.mjs Step 2 INSERT use `manifest.id` for the D1 id column. v1.0.12 reconciles the source `manifest.id` to the catalog authoritative value (Row-5-equivalent for consumers) and republishes. Per Adj-1: no re-tag of v1.0.11; the orphaned GitHub release artefact stays inert (no D1 versions row, no consumer pinned it).

## v1.0.11
GH#645 Row 3b — migrate to K-037 dep-referenced schema. Strip 5 inline shared-content files and declare 4 hub-shared deps (UUID id + slug name + version + checksum from `gen-dep-checksums.mjs`). Internal slug references rewritten for E2 rename/mirror-drop pair(s): report-synthesis→synthesise-weekly-report, synthesise-report→synthesise-weekly-report. Closes pre-Step-3 inline-vendoring for this bundle.

## v1.0.10
Wave 2: re-signed with canonical engine signing pipeline.

## v1.0.9
Tags migrated inline into manifest (GH#586). tags.yaml retired.

## v1.0.8
Bundle re-signed with canonical engine signing pipeline (Wave 2 migration).

## v1.0.7
Signature fix — RELEASE_NOTES.md now included in integrity checksum.

## v1.0.6
Initial catalog release with full structural and content-quality validation. All scanner checks pass.
