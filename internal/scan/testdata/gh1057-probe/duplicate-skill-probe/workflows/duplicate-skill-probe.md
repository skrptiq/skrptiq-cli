---
type: workflow
id: duplicate-skill-probe
title: Duplicate Skill Probe
description: "Four execution entries that produce three steps: a duplicate skill collapses and a prompt-only entry sorts last"
tags: [Fixture]
connections:
  - target: alpha-skill
    type: uses
  - target: beta-skill
    type: uses
  - target: loner-prompt
    type: uses
metadata:
  trigger: manual
output_step: "beta-skill"
execution:
  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "outline-alpha"
    output: { name: "outline", type: "text" }
  - prompt: "loner-prompt"
    step_type: "generation"
    output: { name: "loner_out", type: "text" }
  - skill: "alpha-skill"
    step_type: "generation"
    prompt: "draft-alpha"
    output: { name: "draft", type: "text" }
  - skill: "beta-skill"
    step_type: "generation"
    prompt: "beta-prompt"
    output: { name: "review", type: "text" }
---

## Overview

A probe, not a product. The authored stages read: outline, plan, draft, review.
The engine runs three steps — the outline entry is overwritten by the draft
entry because both name `alpha-skill`, and the prompt-only plan step sorts last.

## Pipeline Stages

### Stage 1: Outline (never runs)
Invoke the **alpha-skill** skill via the **outline-alpha** prompt.

### Stage 2: Plan
Invoke the **loner-prompt** prompt directly, with no backing skill.

### Stage 3: Draft
Invoke the **alpha-skill** skill via the **draft-alpha** prompt.

### Stage 4: Review
Invoke the **beta-skill** skill via the **beta-prompt** prompt.
