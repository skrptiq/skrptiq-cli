---
type: prompt
id: review-idea
title: Review Idea
description: "Presents the idea validation report to the user for approval before design begins"
tags: [Production, Gate, Stage-1]
connections:
  - target: idea-gate
    type: derived_from
metadata:
  output_format: plain
  prompt_type: gate
---

## Purpose

Gate prompt for Stage 1. Presents the idea validation report and collects the user's approval or corrections.

## Prompt

The idea analysis is complete. Review the validation report below and decide whether to proceed to the design stage.

### Validation Report

{{steps.Idea Analysis.output}}

---

### What to Check

1. **Restated goal** — does it accurately capture what you want?
2. **Feasibility** — do the caveats (if any) matter to you?
3. **Node inventory** — are the proposed skills and prompts what you expected?
4. **Pipeline pattern** — does the recommended architecture feel right?
5. **Complexity** — is the scope manageable, or should it be simplified?

### How to Respond

- **To approve:** Type "approved" — the validated idea passes to the Skrpt Design stage as-is
- **To correct:** Describe what needs changing — the corrections will be incorporated into the design brief
- **To stop:** Type "stop" if the idea needs fundamental rethinking

Your response here becomes the input to Stage 2 (Skrpt Design). Be specific about any changes you want.
