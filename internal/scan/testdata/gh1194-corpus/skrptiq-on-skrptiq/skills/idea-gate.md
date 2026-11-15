---
type: skill
id: idea-gate
title: Idea Gate
description: "Human gate — pauses for the user to review the validated idea and approve or redirect before design begins"
tags: [Production, Gate, Stage-1]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Pauses the workflow and presents the idea validation report to the user for review. The user either approves the validated idea or provides corrections before the design stage begins.

## What Happens

1. Execution pauses with status `awaiting_input`
2. The user sees the structured validation report: restated goal, feasibility verdict, node inventory, pipeline pattern, overlap findings, complexity rating
3. The user responds with approval or feedback
4. The response becomes this step's output — passed to Stage 2 (Skrpt Design) as input

## Review Guidance

- Is the restated goal accurate? Does it capture what you actually want?
- Is the recommended pipeline pattern right for your use case?
- Are the identified node types correct — anything missing or unnecessary?
- If there's catalog overlap, do you still want to proceed (customization, different approach)?
- Is the complexity rating acceptable, or should the scope be simplified?

## How to Respond

- **Approve:** Type "approved" or confirm the analysis is correct. The validated idea passes to Stage 2 as-is.
- **Correct:** Provide specific feedback — the analysis will be adjusted before passing to design.
