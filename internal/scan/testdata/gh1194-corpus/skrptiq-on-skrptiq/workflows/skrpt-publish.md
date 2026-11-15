---
type: workflow
id: skrpt-publish
title: Skrpt Publish
description: "Validates the generated skrpt for Hub publication — deterministic checks first, then semantic quality checks, with a final approval gate"
tags: [Production, Builder, Stage-4]
connections:
  - target: publish-check
    type: uses
  - target: publish-gate
    type: uses
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
metadata:
  estimated_duration: "2-5 minutes"
  trigger: manual
  stage: 4
  chain_position: last
output_step: "publish-gate"
composite_steps:
  - "publish-check"
execution:
  - skill: "publish-check"
    prompt: "check-publication"
    step_type: "validation"
    output: { name: "publish_check", type: "decision" }
  - skill: "publish-gate"
    prompt: "review-publication"
    step_type: "review"
    gate: true
    output: { name: "publish_verdict", type: "decision" }
---

## Overview

Stage 4 of the Skrptiq-on-Skrptiq pipeline — the final stage. Takes the approved build from Stage 3 and performs a comprehensive publication readiness check. If approved, the skrpt is ready to save and publish to the Hub.

**Context technique:** This stage front-loads deterministic validation (structure, counts, IDs) before using the LLM for semantic checks (prompt quality, naming, documentation). The mechanical checks are cheap and definitive; the LLM handles the checks that require judgement.

## Pipeline

### Step 1: Publish Check

**Skill:** publish-check | **Prompt:** check-publication

Runs a two-phase validation:
- **Phase 1 (Deterministic):** Connection integrity, manifest counts, ID format, variable resolution, required fields — pass/fail
- **Phase 2 (Semantic):** Prompt quality, description clarity, naming conventions, British English, documentation completeness — pass/warn/fail

**Input:** The approved build output from Stage 3.

**Output:** Publication readiness report with per-check results, issues list, and overall verdict.

### Step 2: Publish Gate

**Skill:** publish-gate | **Prompt:** review-publication

Execution pauses. The user reviews the publication readiness report. Responds with approval (skrpt is ready) or notes for revision (return to Stage 3).

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.generated_skrpt}}` | Yes | The approved build from Stage 3 | Paste the generated file set |

## Outputs

| Name | Description |
|------|-------------|
| Publication approval | The user's final approval — the skrpt is ready to save and publish |

## Chaining

This is **Stage 4** — the final stage. Input comes from Stage 3 (Skrpt Build). After approval, save the generated files as a workspace and publish to the Hub.

## After This Stage

1. Save the generated files from Stage 3 as a skrptiq workspace
2. Run the skrpt locally to test the pipeline end-to-end
3. Publish to the Hub catalog

## Provider Notes

- Single LLM call for the publication check; the gate is human-only
- The deterministic checks don't need the LLM — they're structural validation
- The semantic checks benefit from a model with good code/document analysis capabilities
