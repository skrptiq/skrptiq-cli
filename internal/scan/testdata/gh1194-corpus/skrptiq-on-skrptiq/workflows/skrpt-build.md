---
type: workflow
id: skrpt-build
title: Skrpt Build
description: "Takes an approved design and generates all node files, validates structure, and iterates until the build passes review"
tags: [Production, Builder, Stage-3]
connections:
  - target: node-generation
    type: uses
  - target: build-validation
    type: uses
  - target: build-gate
    type: uses
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
metadata:
  estimated_duration: "5-15 minutes"
  trigger: manual
  stage: 3
  chain_position: middle
loops:
  - id: "build-review"
    mode: "until_pass"
    steps:
      - "node-generation"
      - "build-validation"
      - "build-gate"
    verifier: "build-gate"
    maxIterations: 3
    freshContextPerIteration: true
output_step: "build-gate"
composite_steps:
  - "node-generation"
  - "build-validation"
execution:
  - skill: "node-generation"
    prompt: "generate-nodes"
    step_type: "generation"
    output: { name: "nodes", type: "text" }
  - skill: "build-validation"
    prompt: "validate-build"
    step_type: "validation"
    output: { name: "build_validation", type: "decision" }
  - skill: "build-gate"
    prompt: "review-build"
    step_type: "review"
    gate: true
    output: { name: "build_verdict", type: "decision" }
---

## Overview

Stage 3 of the Skrptiq-on-Skrptiq pipeline. Takes the approved design specification from Stage 2 and generates every file in the skrpt package. The build runs in an iterative loop: generate → validate → review. The loop continues until the user approves the output or the maximum of 3 iterations is reached.

**Context technique:** This stage expects the design specification as structured data (JSON or YAML), not free-text prose. The generator parses the specification directly — each node entry becomes a file.

## Pipeline

### Steps 1–3: Build-Review Loop (until_pass, max 3 iterations)

#### Step 1: Node Generation

**Skill:** node-generation | **Prompt:** generate-nodes

Generates all node files from the design specification. On the first iteration, generates from scratch. On subsequent iterations, revises based on the reviewer's feedback while preserving working parts.

**Input:** The approved design from Stage 2 (structured JSON/YAML).

**Output:** Complete file set as fenced code blocks, each labeled with its path.

#### Step 2: Build Validation

**Skill:** build-validation | **Prompt:** validate-build

Validates the generated files against structural rules: connection integrity, ID format, input declarations, execution block completeness, manifest counts, loop configuration.

**Output:** Structured validation report with pass/continue/fail status and specific issues.

#### Step 3: Build Gate (Loop Verifier)

**Skill:** build-gate | **Prompt:** review-build

Execution pauses. The user reviews the generated files and validation results. Responds with approval (exits loop) or feedback (triggers next iteration).

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.approved_design}}` | Yes | Structured design from Stage 2 | The design specification JSON/YAML |

## Outputs

| Name | Description |
|------|-------------|
| Generated skrpt | Complete file set — all skills, prompts, workflow, manifest, tags, services |

## Chaining

This is **Stage 3** of a four-stage pipeline. Input comes from Stage 2 (Skrpt Design). After approval, copy the generated files and paste into the **Skrpt Publish** workflow's `generated_skrpt` input.

## The Build Loop

- **`until_pass` mode** — loops until the user approves
- **Maximum 3 iterations** — prevents infinite revision cycles
- **Fresh context per iteration** — each revision starts clean with only the design spec + previous feedback
- **The user is the verifier** — their approval/feedback controls the loop

## Provider Notes

- Each iteration invokes the LLM twice (generation + validation); the gate is human-only
- Generation is the most token-intensive step (4,000+ tokens for complex skrpts)
- Validation is lightweight (1,500 tokens) — it checks structure, not semantic quality
