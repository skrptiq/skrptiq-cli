---
type: skill
id: build-gate
title: Build Gate
description: "Human gate — pauses for the user to review generated files and validation results, serves as loop verifier"
tags: [Production, Gate, Loop, Stage-3]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Pauses the workflow and presents the generated skrpt files alongside the validation results. The user reviews both and either approves the build or provides feedback for another iteration.

This skill serves as the **loop verifier** for the build-review loop. The user's response determines whether the loop continues (feedback → re-generate) or exits (approval → proceed to Stage 4).

## What Happens

1. Execution pauses with status `awaiting_input`
2. The user sees: generated files (code blocks) and validation results (pass/continue/fail with issues)
3. The user responds with approval or feedback
4. If approved, the loop exits and the build output passes to Stage 4
5. If feedback is provided, the loop returns to node generation with the feedback incorporated

## Review Guidance

- Are the generated prompts specific enough to produce useful output?
- Do the skill descriptions accurately reflect what each step does?
- Is the workflow's execution order correct?
- Does the manifest look complete?
- Are the validation issues (if any) acceptable or do they need fixing?

## How to Respond

- **To approve:** Type "approved" — the build exits the loop and passes to Stage 4 (Publish)
- **To revise:** Describe what needs changing — the generator will incorporate your feedback in the next iteration
- **To stop:** Type "stop" if the build has fundamental issues requiring a return to design
