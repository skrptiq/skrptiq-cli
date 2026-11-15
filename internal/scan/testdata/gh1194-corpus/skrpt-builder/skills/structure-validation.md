---
type: skill
id: structure-validation
title: Structure Validation
description: "Validates the generated skrpt files against format rules and reports issues"
tags: [Production]
connections:
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
---

## Capability

Validates the generated skrpt output against the format rules from the authoring reference. Produces a structured pass/continue/fail verdict that drives the build-review loop.

## When to Use

After node generation, before the user review gate. Part of the build-review loop.

## What It Does

1. **Check connections** — every prompt has `derived_from` to its skill, every skill has `runs_on` to a service
2. **Check inputs** — every `{{input.*}}` in prompt bodies has a matching `inputs` frontmatter entry with `label` and `description`
3. **Check execution** — every execution entry has `skill`, `prompt`, and `step_type`; referenced skills and prompts exist
4. **Check contents counts** — manifest `contents` matches actual file count
5. **Check IDs** — all kebab-case, no duplicates, types match directories
6. **Check loops** — if present, in workflow frontmatter only; verifier in steps list; maxIterations set
7. **Check data flow** — later-stage prompts reference `{{steps.previous.output}}` or named step output

## Outputs

Validation verdict: `{status: "pass"|"continue"|"fail", reason: "summary", instructions: "specific fixes needed"}`
