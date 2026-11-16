---
type: skill
id: translation-reporting
title: Translation Reporting
description: "Produces a comparative quality matrix across all translated languages with recommendations"
tags: [Production, Reporting]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Aggregates quality review results from all language translations and produces a comparative report. Ranks languages by translation quality, identifies common issues, and recommends which translations are ready for publication and which need further work.

## When to Use

- After completing a batch of translations across multiple languages
- As the post-loop step in a for_each translation workflow
- When stakeholders need a summary of multi-language translation status

## Inputs

- All translation results from the loop (via loop output references)
- The loop status and iteration count

## Outputs

A comparative translation report:
- **Quality matrix** — scores per language per dimension
- **Rankings** — languages ordered by overall quality
- **Common issues** — patterns that appear across multiple languages
- **Publication readiness** — which translations are ready, which need revision
- **Effort estimates** — how much revision work each language needs
