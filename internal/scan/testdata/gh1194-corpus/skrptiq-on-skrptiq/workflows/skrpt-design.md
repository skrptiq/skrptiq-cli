---
type: workflow
id: skrpt-design
title: Skrpt Design
description: "Takes a validated idea and produces a complete architecture: node graph, execution plan, connections, and input/output contract"
tags: [Production, Builder, Stage-2]
connections:
  - target: architecture-design
    type: uses
  - target: design-gate
    type: uses
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
  - target: skrpt-patterns-reference
    type: references
metadata:
  estimated_duration: "3-5 minutes"
  trigger: manual
  stage: 2
  chain_position: middle
output_step: "design-gate"
composite_steps:
  - "architecture-design"
execution:
  - skill: "architecture-design"
    prompt: "design-architecture"
    step_type: "generation"
    output: { name: "architecture", type: "text" }
  - skill: "design-gate"
    prompt: "review-design"
    step_type: "review"
    gate: true
    output: { name: "design_verdict", type: "decision" }
---

## Overview

Stage 2 of the Skrptiq-on-Skrptiq pipeline. Takes the validated idea from Stage 1 and produces a complete, structured design specification. The output is a machine-readable blueprint — not prose — that Stage 3's generator can consume directly.

**Context technique:** This stage expects a summarized validated idea, not the full Stage 1 conversation. Paste the structured analysis (restated goal, node inventory, pipeline pattern, complexity) — not the raw transcript.

## Pipeline

### Step 1: Architecture Design

**Skill:** architecture-design | **Prompt:** design-architecture

Produces the complete design specification: every node defined (type, ID, title, connections, metadata), the execution plan, loop configuration, input/output contract, and manifest values. Output is structured JSON.

**Input:** The validated idea summary from Stage 1.

**Output:** Structured design specification in JSON format.

### Step 2: Design Gate

**Skill:** design-gate | **Prompt:** review-design

Execution pauses. The user reviews the design specification — node inventory, execution order, connections, input contract. Responds with approval or corrections.

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.validated_idea}}` | Yes | Validated idea from Stage 1 | Paste the output from Idea Validation |

## Outputs

| Name | Description |
|------|-------------|
| Design specification | Structured JSON blueprint — input to Stage 3 |

## Chaining

This is **Stage 2** of a four-stage pipeline. Input comes from Stage 1 (Idea Validation). After approval, copy the design specification and paste into the **Skrpt Build** workflow's `approved_design` input.

## Provider Notes

- Single LLM call for the design step; the gate step is human-only
- Design generation benefits from a model with strong structured output (JSON) capabilities
- Output is typically 2,000-3,000 tokens of structured JSON
