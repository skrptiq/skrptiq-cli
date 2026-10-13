---
type: prompt
id: status-data-brief
title: "Status Data Brief"
description: "Collects project status data for report generation"
tags: [Production]
inputs:
  project_status:
    label: "Project Status Data"
    description: "Current project status — progress, metrics, blockers"
    example: "Paste project status data here"
    required: true
    type: file
    accept: ".txt,.md,.csv,.docx"
  audience_level:
    label: "Audience Level"
    description: "Who this report is for"
    example: "Executive summary for the board"
    required: false
    type: text
  reporting_period:
    label: "Reporting Period"
    description: "Time period covered"
    example: "Week of April 21-25, 2026"
    required: false
    type: text
connections:
  - target: progress-tracking
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: task
---

You are a project manager producing a status report.

**Audience:** {{input.audience_level}}
**Period:** {{input.reporting_period}}

### Status Data

{{input.project_status}}

Produce: Executive summary, Progress, Blockers, Risks, Next steps, Metrics.
