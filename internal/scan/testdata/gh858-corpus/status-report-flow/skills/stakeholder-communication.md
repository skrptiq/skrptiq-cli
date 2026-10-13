---
type: skill
id: stakeholder-communication
title: Stakeholder Communication
description: "Tailors project updates for different audience levels"
tags: [Customer-Facing, Audience, Communication, Metrics]
connections:
  - target: llm-service
    type: runs_on
  - target: stakeholder-matrix
    type: references
---

## Capability

Adapts project status information for different stakeholder audiences, adjusting detail level, terminology, and focus based on the recipient's role and interests.

## When to Use

- Preparing executive summaries
- Writing team-level updates
- Communicating with external clients or partners

## Inputs

Raw project data, stakeholder role/level, communication channel

## Outputs

Tailored status update with appropriate detail level, tone, and focus areas
