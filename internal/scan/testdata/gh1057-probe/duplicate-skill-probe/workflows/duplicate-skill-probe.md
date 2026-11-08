---
type: workflow
id: duplicate-skill-probe
title: Duplicate Skill Probe
description: "Four execution entries that produce four steps: a duplicate skill makes two steps sharing one title, and a prompt-only entry sorts last"
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
The engine runs four steps — the outline and draft entries each get their own
step since app GH#1092, and the prompt-only plan step sorts last. Both
alpha-skill steps carry the title `Alpha Skill`, which is the only handle a
`from_step` has, so nothing downstream can address either of them separately.
That is what `workflow.duplicate_skill_entry` refuses.

## Pipeline Stages

### Stage 1: Outline (runs, but nothing can bind to it)
Invoke the **alpha-skill** skill via the **outline-alpha** prompt.

### Stage 2: Plan
Invoke the **loner-prompt** prompt directly, with no backing skill.

### Stage 3: Draft
Invoke the **alpha-skill** skill via the **draft-alpha** prompt.

### Stage 4: Review
Invoke the **beta-skill** skill via the **beta-prompt** prompt.
