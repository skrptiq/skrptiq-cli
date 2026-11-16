---
type: prompt
id: profile-competitor
title: Profile Competitor
description: "Produces a structured competitive profile for a single competitor"
tags: [Production, Competitive, Research]
inputs:
  competitors:
    label: "Competitors"
    description: "JSON array of competitor names to analyze, e.g. [\"Notion AI\", \"Jasper\", \"Copy.ai\"]"
    example: '["Notion AI", "Jasper", "Copy.ai", "Writer"]'
    required: true
    type: longtext
  your_product:
    label: "Your Product"
    description: "Your product name and brief description"
    example: "Skrptiq — AI workflow builder for structured content production"
    required: true
    type: text
  analysis_focus:
    label: "Analysis Focus"
    description: "What aspects to focus the analysis on"
    example: "Focus on pricing models, enterprise features, and developer experience"
    required: true
    type: text
connections:
  - target: competitor-profiling
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: core
---

## Purpose

Drives the competitor profiling skill. Runs once per competitor inside a for_each loop. Produces a structured profile using a consistent format so all competitors are directly comparable.

## Prompt

You are a competitive intelligence analyst. Analyze the following competitor and produce a structured profile.

### Context

- **Your product:** {{input.your_product}}
- **Analysis focus:** {{input.analysis_focus}}

### Competitor

**Name:** {{loop.item}}

This is competitor {{loop.index}} of {{loop.total}} in this analysis batch.

### Instructions

Research and analyze this competitor. Produce a structured profile covering every section below. Be specific — use concrete details, not vague generalisations.

**1. Identity**

| Field | Value |
|-------|-------|
| Company | [Name] |
| Founded | [Year] |
| Headquarters | [City, Country] |
| Team size | [Estimate] |
| Funding | [Stage and total, or "bootstrapped" / "public"] |

**2. Positioning**

- **Value proposition:** One sentence — what do they promise customers?
- **Tagline:** Their actual tagline or marketing headline
- **Market segment:** Who they primarily target

**3. Product**

- **Core features:** 5-8 key capabilities
- **Key differentiators:** What makes them distinct from similar products?
- **Platform:** Web, desktop, mobile, API — what's available?

**4. Target Market**

- **Company size:** Startup / SMB / Mid-market / Enterprise
- **Industries:** Which verticals they focus on
- **Buyer persona:** Who makes the purchase decision?

**5. Pricing**

- **Model:** Freemium / Subscription / Usage-based / One-time
- **Starting price:** Lowest paid tier
- **Enterprise tier:** Available? Custom pricing?
- **Free tier:** What's included?

**6. Strengths**

3-5 specific strengths, each with a brief explanation of why it matters.

**7. Weaknesses**

3-5 specific weaknesses or gaps, each with evidence (user complaints, missing features, market perception).

**8. Threat Assessment**

- **Threat level:** Low / Medium / High — relative to your product
- **Reasoning:** Why this competitor is or isn't a significant threat
- **Overlap:** Where do they compete directly with your product?
- **Divergence:** Where do they serve a different need?

## Formatting Rules

- Use British English throughout
- Use the exact section structure above — do not skip sections or reorder them
- Be specific: "integrates with Slack, Jira, and GitHub" — not "integrates with popular tools"
- If information is not publicly available, state that explicitly rather than guessing
