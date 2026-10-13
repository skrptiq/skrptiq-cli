---
type: workflow
id: status-report-flow
title: Status Report Flow
description: "Gathers data, analyzes progress, and produces stakeholder updates"
tags: [Production, Customer-Facing, Communication, Metrics]
connections:
  - target: progress-tracking
    type: uses
  - target: stakeholder-communication
    type: uses
  - target: llm-service
    type: runs_on
  - target: status-report-template
    type: references
  - target: language-polish
    type: uses
  - target: consistency-check
    type: uses
  - target: visual-spec-generation
    type: uses
output_step: "visual-spec-generation"
composite_steps:
  - "progress-tracking"
  - "stakeholder-communication"
  - "language-polish"
  - "consistency-check"
  - "visual-spec-generation"
execution:
  - skill: "progress-tracking"
    step_type: "synthesis"
    prompt: "status-data-brief"
    output: { name: "status_data", type: "text" }
  - skill: "stakeholder-communication"
    step_type: "generation"
    prompt: "craft-stakeholder-update"
    output: { name: "stakeholder_update", type: "text" }
  - skill: "language-polish"
    prompt: "polish-language"
    step_type: "content"
    output: { name: "polished_report", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      grammar_strictness: "Professional"
  - parallel:
    - skill: "visual-spec-generation"
      prompt: "generate-visual-spec"
      step_type: "synthesis"
      output: { name: "visual_spec", type: "text" }
    - skill: "consistency-check"
      prompt: "check-consistency"
      step_type: "review"
      output: { name: "consistency_verdict", type: "decision" }
      context:
        voice_profile: "Neutral professional tone"
        consistency_strictness: "Standard"
---

## Overview

This workflow collects project data, analyzes progress against milestones, and produces tailored status updates for different stakeholder audiences.

## Pipeline Stages

### Stage 1: Data Gathering

Invoke the **progress-tracking** skill to synthesize velocity, burndown, and milestone data from the project management tooling.

### Stage 2: Progress Analysis

Invoke the **track-progress** prompt to analyze gathered data and identify trends, risks, and highlights worth reporting.

### Stage 3: Stakeholder Update

Invoke the **stakeholder-communication** skill to tailor the update for different audience levels (executive, team lead, team member).

### Stage 4: Email Generation

Invoke the **stakeholder-update-email** prompt to produce a polished, ready-to-send status update email.

## Output

Stakeholder-appropriate status updates containing:

- Progress summary against sprint and project milestones
- Key achievements and blockers
- Risk flags with mitigation actions
- Next steps and upcoming milestones

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.sprint_data}}` | Yes | Sprint or project metrics (velocity, burndown, milestones) | "Sprint 14: 34/40 points completed, 2 stories carried over" |
| `{{input.audience_level}}` | Yes | Stakeholder audience level | executive, team lead, or team |

## Outputs

| Name | Description |
|------|-------------|
| Status update | A tailored status report matched to the audience level |
| Ready-to-send email | The report formatted as an email, ready to copy and send |

## Setup

Before running this workflow:

1. **Jira Cloud access** — if pulling live project data, connect your Jira instance; otherwise paste sprint metrics manually

No specific AI provider or API key is required beyond your configured skrptiq LLM provider.

## Provider Notes

- Works well with any model — status reports are straightforward summarization tasks
- For executive-level reports, the model benefits from clear context about what leadership cares about

## Example Input

To test this workflow immediately after import:

```
Sprint data: Sprint 14 — 34 of 40 story points completed. 2 stories carried over (auth refactor, search indexing). Velocity trending up from 28 to 34 over last 3 sprints. Release v2.1 on track for 28 March.
Audience level: executive
```
