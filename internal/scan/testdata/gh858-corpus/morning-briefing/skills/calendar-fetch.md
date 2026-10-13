---
type: skill
id: calendar-fetch
title: Calendar Fetch
description: "Retrieves today's calendar events from Google Calendar via MCP"
tags: [Production, Calendar]
connections:
  - target: google-calendar-mcp
    type: runs_on
  - target: llm-service
    type: runs_on
---

## Capability

Fetches today's calendar events from Google Calendar using the Calendar MCP server. Retrieves event times, titles, attendees, locations, and descriptions.

## What It Does

1. **Get current time** — calls `get-current-time` to determine the calendar's timezone and today's date
2. **List today's events** — calls `list-events` with today's date range to get the full schedule
3. **Enrich details** — calls `get-event` for events with multiple attendees or long descriptions to get full detail
4. **Structure output** — produces a chronological list of events with time, title, attendees, location, and description

## Outputs

Structured calendar set: chronological list of today's events with time, title, duration, attendees, location, and description.
