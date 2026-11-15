---
type: prompt
id: review-skrpt
title: Review Skrpt
description: "Presents the generated skrpt for user approval or revision"
tags: [Production, Gate]
connections:
  - target: skrpt-review
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Presents the generated skrpt files and validation results to the user for review.

## Prompt

Your skrpt has been generated and validated. Please review the output below.

### Generated Skrpt

{{steps.Node Generation.output}}

### Validation Results

{{steps.Structure Validation.output}}

### What to Check

1. **Prompts** — do the instructions capture what you want? Will the LLM produce good output from these prompts?
2. **Inputs** — are the input fields clear? Will users understand what to provide?
3. **Flow** — does the step order make sense? Is anything missing?
4. **Validation issues** — if any were found, review them. Some may be acceptable, others may need fixing.

### How to Respond

- Type **approved** if the skrpt looks good
- Otherwise, describe what needs to change. Be specific: "the second prompt should also ask about X" or "add a review step after generation"

Your feedback will be used to revise the skrpt in the next iteration.
