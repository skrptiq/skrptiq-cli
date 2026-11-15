---
type: workflow
id: idea-validation
title: Idea Validation
description: "Takes a plain-language skrpt idea, evaluates feasibility, identifies required node types, checks catalog overlap, and assesses complexity"
tags: [Production, Builder, Stage-1]
connections:
  - target: idea-analysis
    type: uses
  - target: idea-gate
    type: uses
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
  - target: skrpt-patterns-reference
    type: references
metadata:
  estimated_duration: "2-5 minutes"
  trigger: manual
  stage: 1
  chain_position: first
output_step: "idea-gate"
composite_steps:
  - "idea-analysis"
execution:
  - skill: "idea-analysis"
    prompt: "analyse-idea"
    step_type: "generation"
    output: { name: "idea_analysis", type: "text" }
  - skill: "idea-gate"
    prompt: "review-idea"
    step_type: "review"
    gate: true
    output: { name: "idea_verdict", type: "decision" }
---

## Overview

Stage 1 of the Skrptiq-on-Skrptiq pipeline. Takes a raw, plain-language idea for a new skrpt and produces a structured validation report. The user reviews the report and either approves or corrects before the idea moves to design.

This stage answers: can this be built? What does it need? Does anything like it already exist?

## Pipeline

### Step 1: Idea Analysis

**Skill:** idea-analysis | **Prompt:** analyse-idea

Evaluates the idea against the skrpt platform's capabilities. Produces a structured report covering: restated goal, feasibility verdict, node type inventory, recommended pipeline pattern, catalog overlap, and complexity assessment.

**Input:** The user's plain-language idea and optionally a list of existing catalog skrpts for overlap checking.

**Output:** Structured validation report with a proceed/simplify/reconsider recommendation.

### Step 2: Idea Gate

**Skill:** idea-gate | **Prompt:** review-idea

Execution pauses. The user reviews the validation report and responds with approval or corrections. This response becomes the output of Stage 1 and the input to Stage 2 (Skrpt Design).

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.idea}}` | Yes | What should your skrpt do? | "A workflow that takes a research paper and generates social media posts" |
| `{{input.existing_skrpts}}` | No | Names of existing catalog skrpts | "blog-post-pipeline, research-paper-pipeline" |

## Outputs

| Name | Description |
|------|-------------|
| Validated idea | The approval response plus the structured validation report — input to Stage 2 |

## Chaining

This is **Stage 1** of a four-stage pipeline. After approval, copy the output and paste it into the **Skrpt Design** workflow's `validated_idea` input.

## Provider Notes

- Single LLM call for the analysis step; the gate step is human-only
- Works well with any capable model — the task is analytical, not generative
