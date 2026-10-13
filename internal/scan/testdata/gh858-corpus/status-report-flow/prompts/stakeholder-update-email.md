---
type: prompt
id: stakeholder-update-email
title: Stakeholder Update Email
description: "Task prompt for generating a formatted status update email"
tags: [Production, Communication, Metrics]
connections:
  - target: stakeholder-communication
    type: derived_from
---

## Purpose

Produces a polished, ready-to-send status update email from analyzed project data.

## Prompt

Compose a professional status update email based on the stakeholder update below. Include a subject line, greeting, progress summary, key achievements, blockers or risks, and next steps. Keep the tone professional but approachable. The email should be scannable with clear sections and bullet points where appropriate.

**Stakeholder update:** {{steps.previous.output}}
