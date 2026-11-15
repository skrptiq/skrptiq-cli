---
type: skill
id: requirements-analysis
title: Requirements Analysis
description: "Analyses the user's goal and produces a structured requirements document with proposed architecture"
tags: [Production]
connections:
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
---

## Capability

Takes the user's plain-language description of what they want their skrpt to do and produces a structured requirements document. Uses the skrpt authoring reference to ground the analysis in what's actually possible.

## When to Use

As the first step in the builder pipeline, before any design or generation work begins.

## What It Does

1. **Restate the goal** — clarify what the user wants in precise terms
2. **Identify node types** — which skills, prompts, services, and sources are needed
3. **Propose the flow** — linear, loop, gated, parallel, or hybrid
4. **Map inputs** — what the generated skrpt will collect from its users
5. **Describe the output** — what the final deliverable looks like

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.goal}}` | Yes | What the skrpt should do | "Review PRs for code style and security" |
| `{{input.audience}}` | No | Who will use the skrpt | "Junior developers" |
| `{{input.inputs_description}}` | No | What the user provides | "A GitHub PR URL" |
| `{{input.complexity}}` | No | simple, medium, or advanced | "medium" |

## Outputs

Structured requirements: restated goal, proposed node list, flow pattern, input variables, output description.
