---
type: skill
id: publish-check
title: Publish Check
description: "Validates the generated skrpt for Hub publication: scanner rules, manifest completeness, prompt quality, and naming conventions"
tags: [Production, Builder, Stage-4]
connections:
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
metadata:
  complexity: medium
  avg_tokens: 2000
---

## Capability

Takes the approved build output from Stage 3 and performs a publication readiness check. Front-loads deterministic validation (structure, counts, IDs) before using the LLM for semantic checks (prompt quality, description clarity, naming conventions).

**Context technique:** This step performs local validation rules first, then uses the LLM for the checks that require judgement. The deterministic checks catch the easy errors; the LLM catches the subtle ones.

## What It Does

### Deterministic Checks (no LLM needed)

1. **Connection integrity** — every `derived_from`, `runs_on`, `uses` edge resolves to an existing node
2. **Manifest counts** — `contents` totals match actual file counts
3. **ID format** — all IDs are lowercase with hyphens, no duplicates
4. **Input variable resolution** — every `{{input.*}}` has a matching `inputs` entry; every `{{steps.*}}` references an existing step
5. **Required fields** — every node has `type`, `id`, `title`; manifest has `name`, `description`, `version`, `licence`

### Semantic Checks (LLM-assisted)

6. **Prompt quality** — are prompts specific enough to produce useful output? Do they include role instructions, clear tasks, and output format guidance?
7. **Description clarity** — are skill and workflow descriptions clear to a user who hasn't seen the design spec?
8. **Naming conventions** — do IDs follow kebab-case, do titles use Title Case, is the package name descriptive?
9. **British English** — are all user-facing strings in British English?
10. **Documentation completeness** — does the workflow body include overview, pipeline stages, inputs table, outputs table?

## Output

Publication readiness report:
- Deterministic check results (pass/fail per check)
- Semantic check results (pass/warn/fail per check)
- Overall verdict: **Ready**, **Ready with warnings**, or **Not ready**
- List of issues to fix (if any)
