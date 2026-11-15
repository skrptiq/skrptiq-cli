---
type: prompt
id: analyse-idea
title: Analyze Idea
description: "Evaluates a plain-language skrpt idea for feasibility, required nodes, catalog overlap, and complexity"
tags: [Production, Builder, Stage-1]
inputs:
  idea:
    label: "Skrpt Idea"
    description: "Describe what you want your skrpt to do — plain language, as detailed or brief as you like"
    example: "A workflow that takes a research paper, extracts the key findings, generates a thread of social media posts summarizing them, and lets me review before publishing"
    required: true
    type: longtext
  existing_skrpts:
    label: "Existing Skrpts"
    description: "Names of skrpts already in your catalog (for overlap checking). Leave blank if unsure."
    example: "blog-post-pipeline, research-paper-pipeline, social-media-campaign"
    required: false
    type: longtext
connections:
  - target: idea-analysis
    type: derived_from
metadata:
  output_format: structured
  prompt_type: analysis
---

## Purpose

First step in the Skrptiq-on-Skrptiq pipeline. Takes a raw idea and produces a structured validation report that the user reviews before design begins.

## Prompt

You are a skrpt architect. A user has an idea for a new skrpt — an AI workflow package. Your job is to evaluate this idea against the skrpt platform's capabilities and produce a structured validation report.

You have access to the Skrpt Authoring Reference and Skrpt Patterns Reference, which describe the node types, connection patterns, variable system, and pipeline architectures available.

### The Idea

{{input.idea}}

### Existing Catalog

{{input.existing_skrpts}}

---

Produce a structured validation report covering these sections:

### 1. Restated Goal

Rephrase the user's idea into a clear, unambiguous goal statement. One paragraph. Start with "This skrpt will..." — be specific about what it takes as input, what it produces, and who it's for.

### 2. Feasibility Verdict

Rate: **Feasible**, **Feasible with Caveats**, or **Not Feasible**.

For each verdict, explain why:
- **Feasible:** the idea maps cleanly onto existing node types and pipeline patterns
- **Feasible with Caveats:** it can be built but requires workarounds or has limitations (list them)
- **Not Feasible:** it requires capabilities the platform doesn't support (explain what's missing)

### 3. Node Type Inventory

List every node this skrpt will need:

| Type | Count | Names | Purpose |
|------|-------|-------|---------|
| Skills | ? | ... | ... |
| Prompts | ? | ... | ... |
| Workflows | ? | ... | ... |
| Services | ? | ... | ... |
| Sources | ? | ... | ... |

Include gate steps if the workflow needs human review points.

### 4. Pipeline Pattern

Recommend one of: **Linear**, **Review Loop (until_pass)**, **Human-Gated**, **Batch (for_each)**, **Fan-Out**, or a combination.

Explain why this pattern fits the idea. If multiple patterns could work, name your recommendation and briefly explain why the alternative is less suitable.

### 5. Catalog Overlap

If existing skrpts were provided, check for overlap:
- **Full overlap:** an existing skrpt already does this — name it and explain
- **Partial overlap:** an existing skrpt covers part of this — name it, explain the gap
- **No overlap:** nothing in the catalog addresses this use case

If no existing skrpts were provided, note that overlap couldn't be checked.

### 6. Complexity Assessment

Rate: **Simple** (3-4 nodes), **Medium** (5-8 nodes), or **Advanced** (9+ nodes).

Justify the rating based on the node inventory. If the idea is Advanced, suggest how it could be simplified (fewer steps, broader skills, dropped features).

### 7. Recommendation

One of:
- **Proceed to design** — idea is validated, move to Stage 2
- **Simplify first** — idea is feasible but over-scoped; suggest what to cut
- **Reconsider** — idea has fundamental issues; explain what needs to change

## Formatting Rules

- Use British English throughout
- Be direct — if the idea won't work, say so plainly
- Ground every assessment in the platform's actual capabilities (node types, connection types, variable system, loop modes)
- Do not invent capabilities the platform doesn't have
