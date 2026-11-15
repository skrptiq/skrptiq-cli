---
type: prompt
id: review-publication
title: Review Publication
description: "Presents the publication readiness report to the user for final approval"
tags: [Production, Gate, Stage-4]
connections:
  - target: publish-gate
    type: derived_from
metadata:
  output_format: plain
  prompt_type: gate
---

## Purpose

Final gate in the Skrptiq-on-Skrptiq pipeline. Presents the publication readiness report and collects the user's final approval.

## Prompt

The publication check is complete. Review the results below and decide whether your skrpt is ready to publish.

### Publication Readiness Report

{{steps.Publish Check.output}}

---

### What to Check

1. **Deterministic checks (Phase 1)** — all five should pass. Any failure here means structural issues that must be fixed.
2. **Semantic checks (Phase 2)** — warnings are advisory but worth considering. Failures need addressing.
3. **Overall verdict** — "Ready" means you can publish. "Ready with warnings" means you can publish but might want to address the warnings first. "Not ready" means there are blocking issues.

### How to Respond

- **To approve:** Type "approved" — your skrpt is publication-ready. Save the generated files from Stage 3 as a workspace and publish to the Hub.
- **To note issues:** Describe what you want to fix — you'll need to return to Stage 3 and regenerate with your feedback.
- **To stop:** Type "stop" if the skrpt needs fundamental rework from an earlier stage.

### What Happens After Approval

Your skrpt is ready to:
1. Save as a workspace in the skrptiq app
2. Run locally to test the pipeline
3. Publish to the Hub catalog for others to discover and import

Congratulations — you just used Skrptiq to build a skrpt for Skrptiq.
