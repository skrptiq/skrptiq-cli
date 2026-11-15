---
type: prompt
id: review-build
title: Review Build
description: "Presents generated files and validation results to the user for approval — serves as loop verifier"
tags: [Production, Gate, Loop, Stage-3]
connections:
  - target: build-gate
    type: derived_from
metadata:
  output_format: plain
  prompt_type: gate
---

## Purpose

Gate prompt and loop verifier for Stage 3. Presents the generated files alongside validation results. The user's response determines whether the loop continues or exits.

## Prompt

The build step is complete. Review the generated files and validation results below.

### Generated Files

{{steps.Node Generation.output}}

### Validation Results

{{steps.Build Validation.output}}

---

### What to Check

1. **Validation status** — if the validator found issues, decide whether they need fixing (provide feedback) or are acceptable (approve anyway)
2. **Prompt quality** — are the prompts specific enough to produce useful AI output? Do they give clear instructions?
3. **Skill descriptions** — do they accurately describe what each step does?
4. **Workflow structure** — is the execution order correct? Are connections complete?
5. **Manifest** — are the contents counts correct? Is the description clear?
6. **Variable references** — do prompts correctly reference upstream step outputs?

### How to Respond

- **To approve:** Type "approved" — the build exits the loop and the files pass to Stage 4 (Publish)
- **To revise:** Describe what needs changing — the generator will incorporate your feedback and produce a new version. Be specific: name the file and what to change.
- **To stop:** Type "stop" if the build needs fundamental changes that require returning to design

This is iteration {{loop.index}} of up to 3. If you provide feedback, the generator will revise and present again.
