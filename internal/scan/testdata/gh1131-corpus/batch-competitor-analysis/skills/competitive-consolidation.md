---
type: skill
id: competitive-consolidation
title: Competitive Consolidation
description: "Combines individual competitor profiles into a comparative matrix and landscape analysis"
tags: [Production, Competitive, Synthesis]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Takes an array of individual competitor analyzes and produces a side-by-side comparison matrix with landscape-level insights. Identifies patterns that are only visible when viewing all competitors together — market clusters, pricing trends, common gaps, and positioning white space.

## When to Use

- As the post-loop consolidation step after a for_each competitor analysis
- When individual analyzes need to be cross-referenced and compared
- When you need to move from "what each competitor does" to "what the landscape looks like"

## Inputs

- Array of competitor profiles (from `{{loop.<id>.results}}`)
- Your product description (for relative positioning)

## Outputs

- A comparison matrix (table format) covering key dimensions across all competitors
- A landscape summary identifying clusters, trends, and gaps
- Your product's relative position in the landscape
