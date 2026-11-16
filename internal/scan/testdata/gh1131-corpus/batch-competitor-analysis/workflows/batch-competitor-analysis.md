---
type: workflow
id: batch-competitor-analysis
title: Batch Competitor Analysis
description: "Analyze a list of competitors individually, then consolidate into a comparative matrix and strategic recommendations"
tags: [Production, Customer-Facing, Competitive, Planning, Loop]
connections:
  - target: competitor-profiling
    type: uses
  - target: competitive-consolidation
    type: uses
  - target: strategic-recommendation
    type: uses
  - target: language-polish
    type: uses
  - target: llm-service
    type: runs_on
  - target: competitor-profile-template
    type: references
metadata:
  estimated_duration: "5-20 minutes"
  trigger: manual
  loop_modes: ["for_each"]
loops:
  - id: "competitor-analysis"
    mode: "for_each"
    inputExpression: "{{input.competitors}}"
    steps:
      - "competitor-profiling"
    maxIterations: 20
output_step: "language-polish"
composite_steps:
  - "competitor-profiling"
  - "competitive-consolidation"
  - "strategic-recommendation"
execution:
  - skill: "competitor-profiling"
    prompt: "profile-competitor"
    step_type: "generation"
    output: { name: "competitor_profile", type: "text" }
  - skill: "competitive-consolidation"
    prompt: "consolidate-analysis"
    step_type: "synthesis"
    output: { name: "comparison_matrix", type: "text" }
  - skill: "strategic-recommendation"
    prompt: "recommend-competitive-strategy"
    step_type: "synthesis"
    output: { name: "recommendations", type: "text" }
  - skill: "language-polish"
    prompt: "polish-language"
    step_type: "content"
    output: { name: "polished_analysis", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      grammar_strictness: "Professional"
---

## Overview

This workflow demonstrates the **for_each** loop pattern. Given a list of competitors, it analyzes each one individually against consistent criteria, then consolidates all analyzes into a comparative matrix with strategic recommendations.

The loop ensures every competitor receives the same depth of analysis. The post-loop consolidation step receives the full results array and produces the comparative output — demonstrating the for_each → consolidate pattern.

## Pipeline Stages

### Stage 1: Individual Analysis (for_each over competitor list)

The loop iterates over the competitor list. For each competitor, a single analysis step produces a structured profile covering:

- Market positioning and value proposition
- Strengths and weaknesses
- Target customers and pricing model
- Key differentiators
- Threat level to your product

Each iteration is independent — the analysis of Competitor B does not depend on or see the analysis of Competitor A. The current competitor name is available via `{{loop.item}}`.

### Stage 2: Consolidation (post-loop)

After all competitors have been analyzed, the consolidation step receives the full results array via `{{loop.competitor-analysis.results}}`. It produces:

- A side-by-side comparison matrix
- Patterns across the competitive landscape
- Your product's relative position

### Stage 3: Strategic Recommendations (post-consolidation)

A final step reads the comparative matrix and produces actionable strategic recommendations: where to differentiate, where to defend, and where to invest.

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.competitors}}` | Yes | List of competitor names (JSON array or one per line) | `["Notion", "Coda", "Slite"]` |
| `{{input.your_product}}` | Yes | Your product name and brief description | "Skrptiq — AI workflow builder for structured content production" |
| `{{input.analysis_focus}}` | No | Specific aspects to emphasize in the analysis | "Focus on AI capabilities, pricing for small teams, and enterprise readiness" |

## Outputs

| Name | Description |
|------|-------------|
| Individual profiles | One structured analysis per competitor |
| Comparison matrix | Side-by-side feature and positioning comparison |
| Strategic recommendations | Where to differentiate, defend, and invest |

## Setup

Before running this workflow:

1. Prepare your competitor list. 3-10 competitors works best. More than 10 produces a very long analysis.
2. Describe your own product clearly — the analysis evaluates competitors relative to your positioning.
3. Optionally specify an analysis focus to get targeted results (e.g. "pricing models" or "enterprise features").

## Provider Notes

- Each competitor triggers one AI call. 5 competitors = 5 calls in the loop, plus 2 post-loop calls (consolidation + recommendations).
- Individual analyzes use temperature 0.3 for consistent, structured output.
- The consolidation step is the most token-intensive — it receives all individual analyzes and must cross-reference them.

## Example Input

To test this workflow immediately after import:

```
competitors: ["Notion", "Coda", "Slite", "Craft", "Obsidian"]
your_product: "A collaborative document editor focused on technical teams, with built-in code blocks, API integrations, and version control"
analysis_focus: "Developer experience, API capabilities, and pricing for teams of 10-50"
```
