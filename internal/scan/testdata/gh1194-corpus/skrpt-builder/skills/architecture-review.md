---
type: skill
id: architecture-review
title: Architecture Review
description: "Human gate — pauses for the user to review the proposed architecture and approve or correct it"
tags: [Production, Gate]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Pauses the workflow and presents the proposed skrpt architecture to the user for review. The user either approves the architecture or provides corrections before generation begins.

## What Happens

1. Execution pauses with status `awaiting_input`
2. The user sees the proposed architecture: skills, flow pattern, inputs, output
3. The user responds with approval or feedback
4. The response becomes this step's output, consumed by the node graph design step

## Review Guidance

- Does the proposed flow pattern make sense for your use case?
- Are the right number of steps proposed?
- Should any skills be added or removed?
- Are the proposed inputs correct — is anything missing?
- Do you want a review loop for quality checking?
