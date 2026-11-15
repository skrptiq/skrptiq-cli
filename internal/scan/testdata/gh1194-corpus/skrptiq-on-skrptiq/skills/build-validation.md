---
type: skill
id: build-validation
title: Build Validation
description: "Validates generated skrpt files against structural rules: connections, IDs, input declarations, execution block integrity"
tags: [Production, Builder, Stage-3]
connections:
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
metadata:
  complexity: medium
  avg_tokens: 1500
---

## Capability

Takes the generated file set from the node generation step and validates it against the skrpt platform's structural rules. Catches errors before the user reviews — wrong connection types, missing `derived_from` edges, undeclared input variables, mismatched contents counts.

## What It Does

1. **Connection integrity** — every prompt has exactly one `derived_from` to its skill; every skill has `runs_on` to a service; every workflow skill has a `uses` edge
2. **ID format** — all IDs are lowercase with hyphens only, no duplicates within the package
3. **Input variable declarations** — every `{{input.*}}` in a prompt body has a matching entry in the prompt's `inputs` frontmatter
4. **Step references** — every `{{steps.*.output}}` references a skill that exists in the execution block
5. **Execution block** — every entry has `skill`, `prompt`, and `step_type`; skills in execution match `uses` connections
6. **Manifest counts** — `contents` counts in `skrptiq.yaml` match the actual number of generated files per type
7. **Loop config** — if loops are declared, all loop step IDs exist, verifier is in the steps list

## Output

Structured validation report:
- `{status: "pass"}` — all checks passed
- `{status: "continue", issues: [...], instructions: "Fix these issues"}` — specific structural issues found
- `{status: "fail", reason: "..."}` — fundamental structural problems that require redesign

Each issue includes: what's wrong, which file, and what the fix should be.
