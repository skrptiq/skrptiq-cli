---
type: prompt
id: validate-structure
title: Validate Structure
description: "Validates generated skrpt files against format rules"
tags: [Production]
connections:
  - target: structure-validation
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Validates the generated skrpt output against the authoring reference rules.

## Prompt

You are a skrpt validator. Check the generated files below against every rule in the authoring reference. Report all issues found.

### Generated Files

{{steps.previous.output}}

### Validation Checklist

Check each rule. For each failure, note the file and specific issue.

**Connections:**
- [ ] Every prompt has exactly one `derived_from` connection to a skill
- [ ] Every skill has `runs_on` connection to a service (usually llm-service)
- [ ] Workflow has `uses` connection to every skill in the execution block
- [ ] Connection types are valid: uses, derived_from, runs_on, references, depends_on, requires, tagged

**Inputs:**
- [ ] Every `{{input.VAR}}` variable in prompt bodies has a matching `inputs` entry in that prompt's frontmatter
- [ ] Every `inputs` entry has `label` and `description` fields
- [ ] At least one prompt declares inputs (the workflow needs a user entry point)

**Execution:**
- [ ] Every execution entry has `skill`, `prompt`, and `step_type`
- [ ] `local.*` step types do NOT need a `prompt` field
- [ ] The `local.*` namespace is **CLOSED**, not open-ended. The engine dispatches exactly seven:
      `local.builtin`, `local.shell`, `local.http`, `local.template`, `local.passthrough`, `local.merge`, `local.select`.
      A `local.*` name outside that list reaches no handler, and **the engine refuses the step at
      dispatch** with an error naming the valid set — so the skrpt fails loudly instead of quietly
      running as an LLM step on a prompt it was never written to send (GH#1010, GH#1011).
- [ ] All referenced skill IDs exist as skill files
- [ ] All referenced prompt IDs exist as prompt files
- [ ] `step_type` is one of the thirteen the engine can execute — LLM-dispatched:
      `generation`, `content`, `review`, `synthesis`, `validation`, `analysis`; or engine-dispatched: `local.builtin`, `local.shell`, `local.http`, `local.template`, `local.passthrough`, `local.merge`, `local.select`.
      **There is no "or any `local.*`".**
- [ ] Parallel blocks have 2+ steps
- [ ] Context bindings must not be empty strings — provide sensible defaults

**Manifest:**
- [ ] `contents` counts match the actual number of files per type
- [ ] `contents.total` equals the sum of all type counts
- [ ] `requires.services` lists all service node IDs
- [ ] `name` is kebab-case

**IDs and Naming:**
- [ ] All node IDs are kebab-case: `[a-z0-9][a-z0-9-]*`
- [ ] No duplicate IDs across the skrpt
- [ ] Node `type` matches its directory (skill in skills/, prompt in prompts/)

**Loops (if present):**
- [ ] Loops are in workflow frontmatter, NOT in skrptiq.yaml
- [ ] Each loop has: id, mode, steps (non-empty), maxIterations
- [ ] until_pass loops have a `verifier` that is in the `steps` list
- [ ] for_each loops have `inputExpression` (guaranteed runtime failure without it)
- [ ] for_each loop step prompts use `{{loop.item}}` to access the current item
- [ ] for_each loops do NOT have a `verifier`

**Data Flow:**
- [ ] Later-stage prompts reference `{{steps.previous.output}}` or `{{steps.Step Title.output}}` or receive data via `{{step.context.*}}`
- [ ] `output_step` references a skill that exists as a file
- [ ] `output_step` should be a generation, synthesis, or content step (not review/validation unless it's a gate)
- [ ] `composite_steps` entries all reference existing skills
- [ ] Pipeline has at least one generation or synthesis step

**Content Quality:**
- [ ] All user-facing text uses British English (analyze not analyze, color not color, organize not organize)
- [ ] Descriptions are specific and meaningful (not "does various things")

### Output Format

```json
{
  "status": "pass|continue|fail",
  "reason": "Summary of validation results",
  "issues": [
    { "file": "path", "rule": "rule name", "detail": "specific issue" }
  ],
  "instructions": "Specific fixes needed for the next generation iteration"
}
```

- **pass**: no issues found — the skrpt is structurally valid
- **continue**: issues found but fixable — list them for the next iteration
- **fail**: fundamental problems (e.g. no workflow, no skills) — needs a redesign
