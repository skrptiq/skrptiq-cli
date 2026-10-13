---
type: prompt
id: fetch-calendar
title: Fetch Calendar
description: "Retrieves today's calendar events from Google Calendar via MCP"
tags: [Production, Calendar]
connections:
  - target: calendar-fetch
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Fetches today's calendar events and structures them for the morning briefing.

## Prompt

You are a data retrieval agent. Using the Google Calendar MCP server, fetch today's schedule.

### Steps

1. Call `get-current-time` to determine the calendar's timezone and today's date
2. Call `list-events` with today's date range (midnight to midnight in the calendar timezone)
3. For events with 3+ attendees or a description longer than 100 characters, call `get-event` to retrieve full details (attendees list, full description, conferencing links)
4. Sort events chronologically

### Output Format

```
date: "2026-04-26"
timezone: "Europe/London"
events:
  - id: "evt123"
    title: "Sprint Planning"
    start: "09:00"
    end: "10:00"
    duration_minutes: 60
    attendees: ["Jane Smith", "Bob Jones", "Lisa Chen"]
    location: "Room 3A / Zoom"
    description: "Review sprint backlog and assign stories"
    is_all_day: false
```

### Error Handling

- If the Calendar MCP server is unreachable, report the error clearly
- If no events are scheduled today, return an empty set with a "clear day" note
