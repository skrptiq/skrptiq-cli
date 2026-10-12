---
type: skill
id: week-data-fetch
title: Week Data Fetch
description: "Retrieves a week's emails and calendar events via MCP in a single pass"
tags: [Production, Email, Calendar]
connections:
  - target: gmail-mcp
    type: runs_on
  - target: google-calendar-mcp
    type: runs_on
  - target: llm-service
    type: runs_on
---

## Capability

Fetches a full week of email and calendar data using both MCP services. Produces a combined dataset for activity analysis.

## What It Does

1. **Fetch emails** — calls `search_emails` with the week's date range to get all sent and received emails
2. **Read key emails** — calls `read_email` on the most relevant threads (replies you sent, important conversations)
3. **Fetch calendar** — calls `list-events` for the week's date range to get all meetings attended
4. **Combine** — produces a unified dataset: emails grouped by thread/topic, calendar events with attendees

## Outputs

Combined week data: email threads with summaries, calendar events with details, and metadata (total emails, meetings attended, threads participated in).
