---
type: prompt
id: fetch-week-data
title: Fetch Week Data
description: "Retrieves a week's emails and calendar events via MCP"
tags: [Production, Email, Calendar]
inputs:
  week_start:
    label: "Week Start Date"
    description: "Start of the reporting week (YYYY-MM-DD). Defaults to last Monday."
    example: "2026-04-20"
    required: false
    type: text
  focus_areas:
    label: "Focus Areas"
    description: "Projects or topics to highlight in the report (comma-separated)"
    example: "Product launch, Q2 planning, Engineering hiring"
    required: false
    type: text
connections:
  - target: week-data-fetch
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Fetches a full week of email and calendar data for the weekly report.

## Prompt

You are a data retrieval agent. Using the Gmail and Google Calendar MCP servers, fetch a week's activity data.

### Steps

1. Determine the date range: if `{{input.week_start}}` is provided, use it as Monday. Otherwise, calculate last Monday's date. The range is Monday 00:00 to Friday 23:59 (or Sunday 23:59 for a full week).

2. **Emails:** Call `search_emails` with `after:{start_date} before:{end_date}` to get the week's emails. Focus on:
   - Emails you sent (to understand what you worked on)
   - Emails you received and replied to (active conversations)
   - Important threads (labeled IMPORTANT or STARRED)
   Limit to 100 results. Call `read_email` on the top 30 most relevant.

3. **Calendar:** Call `list-events` with the week's date range. Call `get-event` for meetings with 3+ attendees to get full details.

4. If `{{input.focus_areas}}` are specified, tag emails and events that relate to those areas.

### Output Format

```
week:
  start: "2026-04-20"
  end: "2026-04-25"

emails:
  sent_count: 45
  received_count: 120
  threads:
    - subject: "Q2 roadmap finalisation"
      participants: ["Sarah", "Mike"]
      your_role: "Contributor — reviewed and approved"
      focus_area: "Q2 planning"
    - ...

calendar:
  meetings_attended: 12
  total_meeting_hours: 9.5
  events:
    - title: "Sprint Planning"
      date: "2026-04-21"
      attendees: ["Team"]
      duration_minutes: 60
    - ...

focus_area_activity:
  "Product launch": { emails: 15, meetings: 3 }
  "Q2 planning": { emails: 8, meetings: 2 }
```
