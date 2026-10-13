---
type: prompt
id: fetch-emails
title: Fetch Emails
description: "Retrieves and structures recent emails from Gmail via MCP"
tags: [Production, Email]
inputs:
  lookback_hours:
    label: "Lookback Hours"
    description: "How many hours of email history to include"
    example: "12"
    required: false
    type: text
  focus_labels:
    label: "Focus Labels"
    description: "Gmail labels to prioritize (comma-separated)"
    example: "INBOX, IMPORTANT"
    required: false
    type: text
connections:
  - target: email-fetch
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Fetches recent emails from Gmail and structures them for urgency triage.

## Prompt

You are a data retrieval agent. Using the Gmail MCP server, fetch recent emails for the morning briefing.

### Steps

1. Call `search_emails` with the query `newer_than:{{input.lookback_hours}}h` (default: `newer_than:12h`). If focus labels are specified in `{{input.focus_labels}}`, add them to the query (e.g. `label:IMPORTANT newer_than:12h`). Set `maxResults` to 50.
2. For each email in the results, call `read_email` with the `messageId` to get the full content: sender, subject, body snippet (first 500 characters), timestamp, labels, and whether it's a reply or part of a thread.
3. Flag emails that likely need a reply: direct questions to you, requests, messages from your manager or direct reports.

### Output Format

```
emails:
  - messageId: "abc123"
    sender: "Jane Smith <jane@example.com>"
    subject: "Q2 planning — need your input by Friday"
    snippet: "Hi, could you review the attached..."
    timestamp: "2026-04-26T07:30:00Z"
    labels: ["INBOX", "IMPORTANT"]
    is_thread: true
    needs_reply: true
```

### Error Handling

- If the Gmail MCP server is unreachable, report the error clearly
- If no emails match the query, return an empty set with a note
