---
type: workflow
id: weekly-report
title: Weekly Report
description: "Fetches a week's emails and calendar via MCP, analyses activity, and generates a stakeholder-ready report"
tags: [Production, Email, Calendar]
connections:
  - target: week-data-fetch
    type: uses
  - target: activity-analysis
    type: uses
  - target: synthesise-weekly-report
    type: uses
  - target: language-polish
    type: uses
  - target: gmail-mcp
    type: runs_on
  - target: google-calendar-mcp
    type: runs_on
  - target: llm-service
    type: runs_on
metadata:
  estimated_duration: "30-90 seconds"
  avg_tokens: 20000
  trigger: manual
output_step: "language-polish"
composite_steps:
  - "week-data-fetch"
  - "activity-analysis"
  - "synthesise-weekly-report"
  - "language-polish"
execution:
  - skill: "week-data-fetch"
    step_type: "generation"
    prompt: "fetch-week-data"
    output: { name: "week_data", type: "text" }
  - skill: "activity-analysis"
    step_type: "synthesis"
    prompt: "analyse-activity"
    output: { name: "activity_analysis", type: "text" }
  - skill: "synthesise-weekly-report"
    step_type: "synthesis"
    prompt: "synthesise-report"
    output: { name: "weekly_report", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      report_format: "Structured markdown"
      report_audience: "General stakeholders"
  - skill: "language-polish"
    step_type: "content"
    prompt: "polish-report"
    output: { name: "polished_report", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      grammar_strictness: "Professional"
---

## Overview

This workflow produces a weekly report from your Gmail and Google Calendar activity. It fetches a week's worth of emails and meetings, analyses themes and accomplishments, and generates a stakeholder-ready report in your voice and format.

Run it every Friday to summarize what you accomplished, what's in progress, and what's blocked — without spending 30 minutes writing it yourself.

## Pipeline Stages

### Stage 1: Week Data Fetch

**Input:** Week start date, focus areas

Using both Gmail and Calendar MCP services, fetch the week's email threads and calendar events. Tags activity related to your specified focus areas.

### Stage 2: Activity Analysis

Analyses the combined data: extracts themes, accomplishments, blockers, key meeting outcomes, and time allocation estimates.

### Stage 3: Report Synthesis

Formats the analysis into a report tailored to your audience (Team, Manager, Executive) in your chosen format (Bullet Points, Narrative, Formal).

### Stage 4: Language Polish

Final cleanup and Voice Profile alignment.

**Output:** Weekly report, ready to share.

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.week_start}}` | No | Reporting week start (YYYY-MM-DD). Default: last Monday. | `2026-04-20` |
| `{{input.focus_areas}}` | No | Projects to highlight (comma-separated). | `Product launch, Q2 planning` |

## Setup

1. **Gmail MCP server** — OAuth 2.0 with `gmail.readonly` scope
2. **Google Calendar MCP server** — OAuth 2.0 with `calendar.readonly` scope

## Example Input

```
Week start: 2026-04-20
Focus areas: Product launch, Q2 planning, Engineering hiring
```
