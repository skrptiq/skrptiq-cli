# Release Notes

## v2.1.1
GH#1143 — remove 3 `default_from_previous` declarations. K-073 revoked the key, and the consumer wave has since bound every slot that carried it, so the fallback can no longer fire: the declaration was inert.

Measured before deleting rather than after: each of the 41 slots across the catalogue was resolved against **every** invocation, not just its carrier's own workflow, and all 41 were bound by every consumer. Nothing here changes behaviour — the fallback had no path to execution.

Removed: `review-translation.translated_text`, `tone-adjustment.translated_text`, `translate-text.translation_brief`.

## v2.1.0
GH#863 Wave 2 (K-045 intent/output-mismatch fix) — wire the never-invoked `translate-text` prompt so the pipeline actually translates. Previously the "translation" step ran only `translation-brief` (a brief, not a translation) and every downstream step consumed the brief. Added a new `text-translation` backing skill and execution step that runs `translate-text` after the brief; rewired tone-adaptation, language-polish, and quality-review to consume the real translation via explicit `from_step` bindings. Re-pinned the `polish-language` shared dep to v1.0.6 and bound its `source`. Contents: skills 3→4, total 9→10.

## v2.0.22
GH#845 — republish with American English (en-US) content, completing the source-only GH#805 flip that never reached the Hub. Copy only — no functional or behaviour change.

## v2.0.21
GH#745 — declare per-step `output: {name, type}` on every execution step (translation/text, toned_translation/text, quality_review/text, translation_report/text, polished_translation/text). Lights up the #744 rich flow-map. Content-only; no bindings or logic changes.

## v2.0.20
GH#645 Row 3b — migrate to K-037 dep-referenced schema. Strip 7 inline shared-content files and declare 7 hub-shared deps (UUID id + slug name + version + checksum from `gen-dep-checksums.mjs`). Closes pre-Step-3 inline-vendoring for this bundle.

## v2.0.19
Wave 2: re-signed with canonical engine signing pipeline.

## v2.0.18
Tags migrated inline into manifest (GH#586). tags.yaml retired.

## v2.0.17
Bundle re-signed with canonical engine signing pipeline (Wave 2 migration).

## v2.0.16
Signature fix — RELEASE_NOTES.md now included in integrity checksum.

## v2.0.15
Initial catalog release with full structural and content-quality validation. All scanner checks pass.
