# Release Notes

## v1.0.4
GH#1100 migration wave (batch 3) — stop reading the upstream value positionally.

The body read `{{steps.previous.output}}` for its **Prior Analysis**, so this prompt received whatever ran immediately before it rather than the value it declares it needs. In a shared prompt that is position-dependence across a reuse boundary: the same prompt sits at a different index in every consumer, so what it got depended on each consumer's ordering.

It now reads `{{step.context.source}}` and declares `source` as a **required** context param. Consumers wire it with `bindings:` (contract §4.3.1); an unbound required slot fails loudly rather than degrading to empty, and there is no `default_from_previous` fallback — revoked on GH#1100.

Consumers are unaffected until each bumps its pin: they continue to resolve this prompt at 1.0.3.

## v1.0.3
GH#845 — republish with American English (en-US) content, completing the source-only GH#805 flip that never reached the Hub. Copy only — no functional or behaviour change.

## v1.0.2
GH#657 Framing B — republish wave. Bundle now ships `dependencies: []` in its signed manifest (injected by `publish-skrpt.mjs` for `--shared` publishes), so the App's dep-referenced install pipeline (post-PR #47) accepts it on standalone update via Hub Update-all. No content changes.

## v1.0.1
GH#645 Row 3a — republish with `manifest.id` aligned to Hub catalog UUID. Pre-K-037 v1.0.0 bundle carried a local `manifest.id` that drifted from the catalog UUID. Row 5 (`0d9c9dbe`) reconciled the source file but not the signed bundle; v1.0.1 ships the corrected `manifest.id` so the batch-of-81 consumer migrations (GH#645 Row 3b) can pin this version and pass engine STEP 4d. No content changes.

## v1.0.0
Initial catalog release as independent shared object (GH#625 Step 2; canonical pick + rename surface per audit).
