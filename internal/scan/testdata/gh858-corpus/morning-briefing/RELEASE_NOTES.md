# Release Notes

## v1.0.14
GH#833 — drop the embedded `gmail-mcp` + `google-calendar-mcp` service nodes; dep-reference the canonicals (`_shared/gmail-mcp@1.0.0`, `_shared/google-calendar-mcp@1.0.0`) — this bundle was the source of the canonical gmail/calendar content. Also fix a pre-existing structural bug the v0.0.21 scanner surfaced: the triage step referenced the prompt slug `triage-email-urgency` as its skill — added a local `email-triage` skill to run it. contents.services 2→0, skills +1.

## v1.0.13
GH#745 — declare per-step `output: {name, type}` on every execution step (emails/text, calendar/text, urgency_triage/text, briefing/text, polished_briefing/text). Lights up the #744 rich flow-map. Content-only; no bindings or logic changes.

## v1.0.12
Fix-forward after Row 3b v1.0.11 publish failure. The v1.0.11 per-skrpt CI's "Register version with Hub API" step failed because the consumer's source `manifest.id` (dbb6aad1…) did not match the D1 catalog row's id (3b8b173d…) — a legacy drift from before Action 6 (`0bcc5ae0`) made publish-skrpt.mjs Step 2 INSERT use `manifest.id` for the D1 id column. v1.0.12 reconciles the source `manifest.id` to the catalog authoritative value (Row-5-equivalent for consumers) and republishes. Per Adj-1: no re-tag of v1.0.11; the orphaned GitHub release artefact stays inert (no D1 versions row, no consumer pinned it).

## v1.0.11
GH#645 Row 3b — migrate to K-037 dep-referenced schema. Strip 4 inline shared-content files and declare 3 hub-shared deps (UUID id + slug name + version + checksum from `gen-dep-checksums.mjs`). Internal slug references rewritten for E2 rename/mirror-drop pair(s): urgency-triage→triage-email-urgency, triage-urgency→triage-email-urgency. Closes pre-Step-3 inline-vendoring for this bundle.

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
