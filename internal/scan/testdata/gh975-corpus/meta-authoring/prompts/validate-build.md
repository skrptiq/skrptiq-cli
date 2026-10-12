---
type: prompt
id: validate-build
title: Validate Build
description: "Validates generated skrpt files against structural rules and reports issues"
tags: [Production, Builder, Stage-3]
connections:
  - target: build-validation
    type: derived_from
metadata:
  output_format: structured
  prompt_type: analysis
---

## Purpose

Validates the generated files from the node generation step. Catches structural errors before the user reviews.

## Prompt

You are a skrpt structure validator. You have a set of generated skrpt files and must check them against the platform's structural rules. Report every issue found — do not fix them, just identify them precisely.

### Generated Files

{{steps.Node Generation.output}}

---

Run these checks in order:

### 1. Connection Integrity

- [ ] Every prompt has exactly one `derived_from` connection pointing to a skill that exists in the package
- [ ] Every skill has a `runs_on` connection pointing to a service that exists in the package
- [ ] The workflow has a `uses` connection for every skill listed in the execution block
- [ ] Gate skills have `metadata: { gate: true }`

### 2. ID Format

- [ ] All node IDs match the pattern `[a-z0-9][a-z0-9-]*` (lowercase, hyphens, no leading hyphen)
- [ ] No duplicate IDs within the package
- [ ] Each node's `id` matches its filename (without `.md`)

### 3. Input Variable Declarations

- [ ] Every `{{input.field_name}}` in a prompt body has a matching entry in that prompt's `inputs` frontmatter with `label` and `description`
- [ ] Only first-step prompts declare `inputs` (later prompts should not)

### 4. Step References

- [ ] Every `{{steps.Step Title.output}}` in a prompt references a skill whose Title Case title matches a skill in the execution block
- [ ] No references to non-existent steps

### 5. Execution Block

- [ ] Every execution entry has `skill`, `prompt`, and `step_type`
- [ ] `local.*` step types do NOT need a `prompt` field
- [ ] The `local.*` namespace is **CLOSED**, not open-ended. The engine dispatches exactly seven:
      `local.builtin`, `local.shell`, `local.http`, `local.template`, `local.passthrough`, `local.merge`, `local.select`.
      A `local.*` name outside that list reaches no handler, and **the engine refuses the step at
      dispatch** with an error naming the valid set — so the skrpt fails loudly instead of quietly
      running as an LLM step on a prompt it was never written to send (GH#1010, GH#1011).
- [ ] Every skill in execution has a matching `uses` connection in the workflow
- [ ] Step types are one of the thirteen the engine can execute — LLM-dispatched:
      `generation`, `content`, `review`, `synthesis`, `validation`, `analysis`; or engine-dispatched: `local.builtin`, `local.shell`, `local.http`, `local.template`, `local.passthrough`, `local.merge`, `local.select`.
      **There is no "or any `local.*`".**
- [ ] Context bindings must not be empty strings — provide sensible defaults

### 6. Manifest Counts

- [ ] `contents.skills` matches the number of skill files
- [ ] `contents.prompts` matches the number of prompt files
- [ ] `contents.workflows` matches the number of workflow files
- [ ] `contents.services` matches the number of service files
- [ ] `contents.sources` matches the number of source files
- [ ] `contents.total` equals the sum of all content counts

### 7. Loop Configuration (if applicable)

- [ ] Every loop has `id`, `mode`, `steps`, `maxIterations`
- [ ] For `until_pass` loops: `verifier` is specified and is in the `steps` list
- [ ] For `for_each` loops: `inputExpression` is present (guaranteed runtime failure without it)
- [ ] For `for_each` loops: at least one loop step prompt uses `{{loop.item}}`
- [ ] All step IDs in the loop exist in the execution block

### 8. Content Quality

- [ ] All user-facing text uses British English (analyze not analyze, color not color, organize not organize)
- [ ] Descriptions are specific and meaningful
- [ ] `output_step` references an existing skill file
- [ ] `output_step` is a generation, synthesis, or content step (not review/validation unless it's a gate)
- [ ] Pipeline has at least one generation or synthesis step

### Output Format

Produce a structured result:

```json
{
  "status": "pass | continue | fail",
  "checks_run": 7,
  "checks_passed": 7,
  "issues": [
    {
      "check": "Connection Integrity",
      "file": "prompts/my-prompt.md",
      "issue": "Missing derived_from connection",
      "fix": "Add connection: { target: my-skill, type: derived_from }"
    }
  ],
  "instructions": "Fix the issues listed above"
}
```

- **pass:** all checks passed, no issues
- **continue:** issues found but they are fixable by regeneration
- **fail:** fundamental structural problems that require returning to the design stage

## Rules

- Check every file, not just a sample
- Report the exact file and line where each issue occurs
- Be precise about what's wrong and what the fix should be
- If everything passes, say so — do not manufacture issues
- Use British English throughout
