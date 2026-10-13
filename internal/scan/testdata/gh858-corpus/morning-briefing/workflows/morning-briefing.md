---
type: workflow
id: morning-briefing
title: Morning Briefing
description: "Pulls emails and calendar via MCP, triages by urgency, and produces a personalised day-ahead briefing"
tags: [Production, Email, Calendar]
connections:
  - target: email-fetch
    type: uses
  - target: calendar-fetch
    type: uses
  - target: email-triage
    type: uses
  - target: briefing-synthesis
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
  estimated_duration: "20-60 seconds"
  avg_tokens: 15000
  trigger: manual
output_step: "language-polish"
composite_steps:
  - "email-fetch"
  - "calendar-fetch"
  - "email-triage"
  - "briefing-synthesis"
  - "language-polish"
execution:
  - parallel:
    - skill: "email-fetch"
      step_type: "generation"
      prompt: "fetch-emails"
      output: { name: "emails", type: "text" }
    - skill: "calendar-fetch"
      step_type: "generation"
      prompt: "fetch-calendar"
      output: { name: "calendar", type: "text" }
  - skill: "email-triage"
    step_type: "synthesis"
    prompt: "triage-email-urgency"
    output: { name: "urgency_triage", type: "text" }
    context:
      urgency_sensitivity: "Standard"
  - skill: "briefing-synthesis"
    step_type: "synthesis"
    prompt: "synthesise-briefing"
    output: { name: "briefing", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      briefing_detail: "Standard"
  - skill: "language-polish"
    step_type: "content"
    prompt: "polish-briefing"
    output: { name: "polished_briefing", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      grammar_strictness: "Professional"
---

## Overview

This workflow produces a personalised morning briefing from your Gmail inbox and Google Calendar. It fetches today's emails and events in parallel, triages emails by urgency, synthesises everything into a readable briefing, and applies a final language polish.

Run it each morning to know exactly what needs your attention, what meetings are ahead, and which emails need replies — all in your voice.

## Pipeline Stages

### Stage 1: Parallel Data Fetch

Two data retrieval agents run concurrently:

#### 1a. Email Fetch

Using the Gmail MCP service, search for recent emails (configurable lookback, default 12 hours) and read their content. Flags emails that likely need replies.

#### 1b. Calendar Fetch

Using the Google Calendar MCP service, fetch today's full schedule with event details, attendees, and descriptions.

### Stage 2: Urgency Triage

Classifies each email into four urgency levels: action required, needs reply, FYI, or skippable. Configurable via the `urgency_sensitivity` persona dial.

### Stage 3: Briefing Synthesis

Combines the triaged emails and calendar events into a single briefing. Connects related items (e.g. emails about a meeting happening today). Detail level controlled by the `briefing_detail` persona dial.

### Stage 4: Language Polish

Final cleanup: spelling, grammar, clarity, and Voice Profile alignment. Uses British English throughout.

**Output:** Your morning briefing, ready to read.

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.lookback_hours}}` | No | Hours of email history. Default: 12. | `24` |
| `{{input.focus_labels}}` | No | Gmail labels to prioritize. | `INBOX, IMPORTANT` |

## Outputs

| Name | Description |
|------|-------------|
| Morning briefing | Day-ahead summary with schedule, action items, replies needed, and FYI digest |

## Setup

Before running this workflow:

1. **Gmail MCP server** — install and configure the Gmail MCP server. Requires OAuth 2.0 credentials with `gmail.readonly` scope.
2. **Google Calendar MCP server** — install and configure the Calendar MCP server. Requires OAuth 2.0 credentials with `calendar.readonly` scope.
3. **Authorise both servers** — on first run, each server will open a browser window for OAuth authorisation.

## Provider Notes

- Email triage is an analytical task — most models handle it well.
- Briefing synthesis benefits from a model with strong writing capabilities.
- The pipeline is moderate on token usage, but high email volumes (50+ emails) will increase consumption.

## Example Input

To test this workflow immediately after import, use **Try with Examples**.

```
Lookback hours: 12
Focus labels: INBOX, IMPORTANT
```
