---
type: prompt
id: consolidate-analysis
title: Consolidate Analysis
description: "Combines individual competitor profiles into a comparative matrix and landscape analysis"
tags: [Production, Competitive, Synthesis]
inputs:
  your_product:
    label: "Your Product"
    description: "Your product name and brief description"
    example: "Skrptiq — AI workflow builder for structured content production"
    required: true
    type: text
connections:
  - target: competitive-consolidation
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: synthesis
---

## Purpose

Drives the competitive consolidation skill. Runs after the for_each loop completes. Receives all individual competitor profiles and produces a comparative view.

## Prompt

You are a market strategist. You have received individual profiles for {{loop.competitor-analysis.iterations}} competitors. Your task is to consolidate them into a comparative analysis.

### Your Product

{{input.your_product}}

### Individual Competitor Profiles

{{loop.competitor-analysis.results}}

### Instructions

Produce a consolidated competitive analysis with three sections:

**1. Comparison Matrix**

Create a markdown table comparing all competitors across these dimensions:

| Dimension | [Competitor 1] | [Competitor 2] | ... | Your Product |
|-----------|----------------|----------------|-----|-------------|
| Primary value prop | | | | |
| Target market | | | | |
| Pricing model | | | | |
| Starting price | | | | |
| Key differentiator | | | | |
| Threat level | | | | |

Add rows for any dimensions that are particularly relevant given the analysis focus.

**2. Landscape Analysis**

Synthesize patterns visible only when viewing all competitors together:

- **Market clusters:** Are competitors grouped around specific approaches or segments?
- **Pricing trends:** Is the market converging on a pricing model? Where are the outliers?
- **Common strengths:** What do most competitors do well? (These are table stakes.)
- **Common gaps:** What does no competitor do well? (These are opportunities.)
- **Emerging patterns:** Any shifts or trends visible across the competitive set?

**3. Relative Position**

Where does your product sit in this landscape?

- **Unique advantages:** What do you offer that no competitor matches?
- **Competitive pressure:** Where are you most vulnerable?
- **White space:** Where could you position that no one currently occupies?

## Formatting Rules

- Use British English throughout
- Use markdown tables for the comparison matrix — they must render cleanly
- Reference specific competitors by name — no vague "some competitors" language
- Keep the landscape analysis to 3-5 bullet points per section
