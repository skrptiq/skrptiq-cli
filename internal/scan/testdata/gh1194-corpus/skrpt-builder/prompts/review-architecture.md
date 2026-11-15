---
type: prompt
id: review-architecture
title: Review Architecture
description: "Presents the proposed architecture for user approval"
tags: [Production, Gate]
connections:
  - target: architecture-review
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Presents the proposed skrpt architecture to the user and collects approval or corrections.

## Prompt

Here is the proposed architecture for your skrpt:

{{steps.previous.output}}

### Review Checklist

Please review the proposed design:

1. **Goal** — does the restated goal capture what you want?
2. **Pipeline pattern** — does the flow make sense? (linear, loop, gated, etc.)
3. **Skills** — are the right capabilities included? Too many? Too few?
4. **Inputs** — will your skrpt collect the right information from users?
5. **Output** — is the described deliverable what you expect?

### How to Respond

- Type **approved** if the architecture looks right
- Otherwise, describe what should change — add steps, remove steps, change the flow, adjust inputs, etc.

Your feedback will be incorporated into the detailed design.
