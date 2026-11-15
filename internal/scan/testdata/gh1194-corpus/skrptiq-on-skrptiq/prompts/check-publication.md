---
type: prompt
id: check-publication
title: Check Publication
description: "Validates the generated skrpt for Hub publication readiness — deterministic checks first, then semantic checks"
tags: [Production, Builder, Stage-4]
inputs:
  generated_skrpt:
    label: "Generated Skrpt"
    description: "The approved build output from Stage 3 (Skrpt Build). Paste the complete generated file set."
    example: "All generated files from the Skrpt Build stage"
    required: true
    type: longtext
connections:
  - target: publish-check
    type: derived_from
metadata:
  output_format: structured
  prompt_type: analysis
---

## Purpose

Final validation step. Takes the generated skrpt files and performs a comprehensive publication readiness check. Deterministic checks first (structure, counts, IDs), then semantic checks (quality, clarity, conventions).

**Context technique:** Front-load deterministic validation before using LLM judgement. Run the mechanical checks first — they're cheap, fast, and definitive. Only then apply the LLM to checks that require interpretation.

## Prompt

You are a skrpt publication reviewer. You have a complete set of generated skrpt files and must determine whether they are ready for Hub publication. Run checks in two phases: deterministic first, semantic second.

### Generated Skrpt Files

{{input.generated_skrpt}}

---

## Phase 1: Deterministic Checks

Run these checks systematically. Each check is pass or fail — no judgement needed.

### D1. Connection Integrity
- Every prompt has exactly one `derived_from` → skill
- Every skill has `runs_on` → service
- Every workflow `uses` → every skill in its execution block
- All connection targets resolve to nodes that exist in the package

### D2. Manifest Counts
- `contents.skills` matches actual skill file count
- `contents.prompts` matches actual prompt file count
- `contents.workflows` matches actual workflow file count
- `contents.services` matches actual service file count
- `contents.sources` matches actual source file count
- `contents.total` equals the sum

### D3. ID Format
- All node IDs match `[a-z0-9][a-z0-9-]*`
- No duplicate IDs within the package
- Package `name` in manifest matches the pattern

### D4. Input Variable Resolution
- Every `{{input.field_name}}` in a prompt body has a matching `inputs` entry
- Every `{{steps.Step Title.output}}` references a skill title that exists in the execution block

### D5. Required Fields
- Every node has `type`, `id`, `title`
- Manifest has `name`, `description`, `version`, `engine`, `licence`, `category`, `requires`, `contents`

### D6. Execution Completeness
- `local.*` step types do NOT need a `prompt` field; all others do
- Context bindings must not be empty strings
- `step_type` is valid: generation, content, review, synthesis, validation, analysis, or `local.*`

### D7. Loop Completeness (if applicable)
- Every loop has `id`, `mode`, `steps` (non-empty), `maxIterations`
- `until_pass` loops have a `verifier` in the `steps` list
- `for_each` loops have `inputExpression`
- `for_each` loop prompts use `{{loop.item}}`

### D8. Pipeline Coherence
- `output_step` references an existing skill file
- `output_step` is a deliverable step (generation, synthesis, content) or a gate
- Pipeline has at least one generation or synthesis step

## Phase 2: Semantic Checks

Run these checks using your judgement. Each check is pass, warn, or fail.

### S1. Prompt Quality
- Do prompts include a clear role instruction ("You are a...")?
- Do they define the task with numbered steps or clear sections?
- Do they specify the expected output format?
- Are they specific enough to produce useful results, or too generic?

### S2. Description Clarity
- Can a user understand what each skill and workflow does from the description alone?
- Are descriptions specific (not "does various things") and accurate?

### S3. Naming Conventions
- Are IDs descriptive and consistent (kebab-case)?
- Are titles in Title Case?
- Is the package name descriptive of what it does?

### S4. British English
- Are all user-facing strings (descriptions, prompt content, documentation) in British English?
- Check for: "analyze" → "analyze", "color" → "color", "optimize" → "optimize", etc.

### S5. Documentation Completeness
- Does the workflow body include: overview, pipeline stages, inputs table, outputs table?
- Do skills have: capability, what it does, output sections?

## Output Format

```json
{
  "phase_1": {
    "D1_connections": "pass | fail",
    "D2_manifest": "pass | fail",
    "D3_ids": "pass | fail",
    "D4_variables": "pass | fail",
    "D5_fields": "pass | fail",
    "issues": []
  },
  "phase_2": {
    "S1_prompts": "pass | warn | fail",
    "S2_descriptions": "pass | warn | fail",
    "S3_naming": "pass | warn | fail",
    "S4_british_english": "pass | warn | fail",
    "S5_documentation": "pass | warn | fail",
    "issues": []
  },
  "verdict": "Ready | Ready with warnings | Not ready",
  "summary": "One paragraph assessment"
}
```

For each issue, include: which check, which file, what's wrong, and how to fix it.

## Rules

- Phase 1 failures are blocking — "Not ready" if any D check fails
- Phase 2 failures are blocking; warnings are advisory
- Be honest — if the skrpt is publication-ready, say so. Don't manufacture issues.
- Use British English throughout your report
