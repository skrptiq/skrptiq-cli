---
type: skill
id: activity-analysis
title: Activity Analysis
description: "Analyses a week's activity — themes, accomplishments, blockers, and time allocation"
tags: [Production, Quality]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Takes the combined week data and extracts structured insights: what you worked on, what you accomplished, what blocked you, and how your time was spent.

## What It Does

1. **Themes** — identifies the main topics/projects you engaged with, ranked by time and email volume
2. **Accomplishments** — things completed, decisions made, deliverables sent
3. **Blockers** — issues raised, questions pending, items waiting on others
4. **Meetings** — time spent in meetings, key meetings with outcomes
5. **Communication** — email volume, key conversations, response patterns
6. **Focus areas** — if the user specified focus areas, highlights those specifically

## Outputs

Structured activity analysis: themes, accomplishments, blockers, meeting summary, and time allocation breakdown.
