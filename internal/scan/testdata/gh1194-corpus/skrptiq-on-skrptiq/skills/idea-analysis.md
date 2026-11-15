---
type: skill
id: idea-analysis
title: Idea Analysis
description: "Evaluates a plain-language skrpt idea: feasibility, required node types, catalog overlap, complexity assessment"
tags: [Production, Builder, Stage-1]
connections:
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
  - target: skrpt-patterns-reference
    type: references
metadata:
  complexity: medium
  avg_tokens: 2000
---

## Capability

Takes a raw, plain-language idea for a skrpt and produces a structured validation report. Answers three questions: can this be built as a skrpt? What does it need? Does anything like it already exist?

## What It Does

1. **Restates the idea** — rephrases the user's description into a clear, unambiguous goal statement
2. **Feasibility check** — determines whether the idea maps onto the skrpt execution model (skills, prompts, workflows, services). Flags anything that requires capabilities outside the platform (e.g. persistent state, real-time streaming, custom UI)
3. **Node type inventory** — identifies which node types are needed: how many skills, whether prompts need user input, whether a loop or gate pattern fits, what services are required
4. **Pipeline pattern match** — recommends a pipeline architecture (linear, review loop, gated, batch, fan-out) based on the idea's structure
5. **Catalog overlap** — checks whether existing Hub skrpts already cover this use case (fully, partially, or not at all)
6. **Complexity assessment** — rates the idea as simple (3-4 nodes), medium (5-8 nodes), or advanced (9+ nodes) with justification

## Output

Structured validation report with:
- Restated goal
- Feasibility verdict (feasible / feasible with caveats / not feasible)
- Node type inventory with counts
- Recommended pipeline pattern
- Overlap findings
- Complexity rating
- Recommendation: proceed to design, simplify first, or reconsider

## Limitations

The analysis is based on the skrpt platform's current capabilities. It cannot assess ideas that depend on features not yet built (e.g. workflow chaining, persistent memory between runs). The catalog overlap check depends on the user providing existing skrpt names — it cannot query the Hub directly.
