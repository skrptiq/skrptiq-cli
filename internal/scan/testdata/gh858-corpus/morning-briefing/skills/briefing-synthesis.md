---
type: skill
id: briefing-synthesis
title: Briefing Synthesis
description: "Combines triaged emails and calendar events into a personalised day-ahead briefing"
tags: [Production, Email, Calendar]
connections:
  - target: llm-service
    type: runs_on
context_params:
  voice_profile:
    label: "Voice Profile"
    description: "Your writing style for the briefing — tone, vocabulary, and sentence patterns"
    required: false
  briefing_detail:
    label: "Briefing Detail"
    description: "How detailed the briefing should be — Brief, Standard, or Detailed"
    default: "Standard"
    required: false
---

## Capability

Merges the urgency-triaged emails and calendar events into a single, readable morning briefing. Prioritizes what matters, connects related items (e.g. an email thread about a meeting happening today), and presents the day ahead clearly.

## What It Does

1. **Day overview** — headline stats: meeting count, email count, urgent items count, free time blocks
2. **Schedule** — today's events in chronological order with prep notes for important meetings
3. **Action items** — emails classified as action-required, with who sent them and what they need
4. **Replies needed** — emails expecting a reply, summarized with suggested priority order
5. **FYI digest** — brief one-line summaries of informational emails worth knowing about
6. **Cross-references** — connects related emails and calendar events (e.g. "You have a 1:1 with Sarah at 2pm — she emailed about the Q3 roadmap yesterday")

## Briefing Detail Levels

- **Brief** — day overview + action items only. 200–400 words. Scan in 30 seconds.
- **Standard** — overview + schedule + action items + replies needed. 400–800 words. Read in 2 minutes.
- **Detailed** — everything including FYI digest and cross-references. 800–1,200 words. Full context.

## Outputs

Formatted morning briefing in markdown.
