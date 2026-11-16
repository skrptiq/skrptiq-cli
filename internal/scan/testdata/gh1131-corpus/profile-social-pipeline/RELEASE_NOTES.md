# Release Notes

## v1.0.19
GH#863 Wave 3 (Tier-3 tail) — the output_step (language-polish) was polishing its positional-previous step (a validation/review), not the deliverable, because polish-language was pinned pre-`source`-slot. Repinned polish-language 1.0.1→1.0.6 and bound language-polish `source` ← "Platform Adaptation" (the real deliverable producer). No new steps; no other change.

## v1.0.18
GH#845 — republish with American English (en-US) content, completing the source-only GH#805 flip that never reached the Hub. Copy only — no functional or behaviour change.

## v1.0.17
GH#745 — declare per-step `output: {name, type}` on every execution step (core_message/text, platform_content/text, voice_verdict/decision, polished_posts/text, consistency_verdict/decision). Lights up the #744 rich flow-map. Content-only; no bindings or logic changes.

## v1.0.16
GH#711 — fix the `for_each` platform loop. The `platform-loop` iterates `{{input.platforms}}`, but the input was documented and exampled as a comma-separated string (`LinkedIn, Twitter, …`), which resolves to prose the loop cannot iterate (the engine now fails this loud — GH#557). The input contract is now a JSON array (or one platform per line), mirroring the proven `batch-competitor-analysis` for_each pattern, so the expression resolves to an iterable array. Content-only: Inputs table, Stage 2 wording (notes `{{loop.item}}`), and Example Input updated; no contract or engine change.

## v1.0.15
Fix-forward after Row 3b v1.0.14 publish failure. The v1.0.14 per-skrpt CI's "Register version with Hub API" step failed because the consumer's source `manifest.id` (5697d19d…) did not match the D1 catalog row's id (912dbfcd…) — a legacy drift from before Action 6 (`0bcc5ae0`) made publish-skrpt.mjs Step 2 INSERT use `manifest.id` for the D1 id column. v1.0.15 reconciles the source `manifest.id` to the catalog authoritative value (Row-5-equivalent for consumers) and republishes. Per Adj-1: no re-tag of v1.0.14; the orphaned GitHub release artefact stays inert (no D1 versions row, no consumer pinned it).

## v1.0.14
GH#645 Row 3b — migrate to K-037 dep-referenced schema. Strip 5 inline shared-content files and declare 5 hub-shared deps (UUID id + slug name + version + checksum from `gen-dep-checksums.mjs`). Closes pre-Step-3 inline-vendoring for this bundle.

## v1.0.13
Wave 2: re-signed with canonical engine signing pipeline.

## v1.0.12
Tags migrated inline into manifest (GH#586). tags.yaml retired.

## v1.0.11
Bundle re-signed with canonical engine signing pipeline (Wave 2 migration).

## v1.0.10
Signature fix — RELEASE_NOTES.md now included in integrity checksum.

## v1.0.9
Initial catalog release with full structural and content-quality validation. All scanner checks pass.
