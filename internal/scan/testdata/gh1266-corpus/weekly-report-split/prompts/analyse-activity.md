---
type: prompt
id: analyse-activity
title: Analyze Activity
description: "Extracts themes, accomplishments, blockers, and time allocation from weekly data"
tags: [Production, Quality]
connections:
  - target: activity-analysis
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Analyses the week's combined email and calendar data to extract structured insights.

## Prompt

You are an activity analysis agent. Analyze the week's data below and extract structured insights.

### What to Extract

1. **Themes** — the 3–5 main topics/projects you engaged with this week, ranked by time invested (estimated from email volume and meeting hours)
2. **Accomplishments** — concrete things completed: decisions made, deliverables sent, milestones reached, problems solved
3. **In progress** — work started but not finished, with estimated completion
4. **Blockers** — items stuck: waiting on someone, missing information, dependencies not met
5. **Key meetings** — the 3–5 most important meetings with outcomes (decisions made, actions assigned)
6. **Time allocation** — estimated split: meetings vs email vs deep work vs admin

### Input

- **Week data:** {{steps.previous.output}}

### Output Format

```
analysis:
  themes:
    - name: "Q2 planning"
      time_estimate: "40%"
      summary: "Finalised roadmap, assigned workstreams"
  accomplishments:
    - "Shipped roadmap v3 to stakeholders"
    - "Closed 3 open hiring decisions"
  in_progress:
    - "API migration — 60% complete, on track for next Wednesday"
  blockers:
    - "Budget approval for contractor — waiting on Finance (since Tuesday)"
  key_meetings:
    - title: "Q2 Roadmap Review"
      outcome: "Approved 3 themes, deferred analytics to Q3"
  time_allocation:
    meetings: "35%"
    email: "25%"
    deep_work: "30%"
    admin: "10%"
```

### Rules

- Only report accomplishments you can evidence from the data — don't infer work that isn't visible in emails or calendar
- If focus areas were specified, ensure they appear prominently in themes
