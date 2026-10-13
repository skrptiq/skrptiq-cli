---
type: skill
id: email-fetch
title: Email Fetch
description: "Retrieves recent emails from Gmail via MCP for the morning briefing"
tags: [Production, Email]
connections:
  - target: gmail-mcp
    type: runs_on
  - target: llm-service
    type: runs_on
---

## Capability

Fetches recent emails from Gmail using the Gmail MCP server. Searches the inbox for messages within the configured lookback period, reads their content, and structures them for downstream analysis.

## What It Does

1. **Search inbox** — calls `search_emails` with a time-based query (e.g. `newer_than:12h`) filtered to the specified labels
2. **Read content** — calls `read_email` for each result to get full message content, sender, subject, and timestamp
3. **Structure output** — produces a list of emails with sender, subject, snippet, timestamp, labels, and whether a reply is expected

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.lookback_hours}}` | No | Hours of email history. Default: 12. | `24` |
| `{{input.focus_labels}}` | No | Gmail labels to prioritize. | `INBOX, IMPORTANT` |

## Outputs

Structured email set: list of messages with sender, subject, snippet, timestamp, labels, and reply-needed flag.
