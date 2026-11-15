---
type: skill
id: skrpt-review
title: Skrpt Review
description: "Human gate and loop verifier — pauses for the user to review the generated skrpt and approve or request changes"
tags: [Production, Gate, Loop]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Pauses the workflow and presents the generated skrpt files and validation results to the user. The user either approves the skrpt or provides feedback for revision.

This is both a **gate step** (pauses for user input) and the **loop verifier** for the build-review loop. If the user approves, the loop exits. If the user provides feedback, the loop returns to node-generation.

## What Happens

1. Execution pauses with status `awaiting_input`
2. The user sees the generated skrpt files and any validation issues
3. The user responds:
   - **"approved"** or similar → loop exits, proceeds to final assembly
   - **Specific feedback** → loop continues, node-generation incorporates the feedback
4. The user's response becomes this step's output

## Review Guidance

- Do the prompts capture your intent?
- Are the inputs right — will users know what to provide?
- Does the workflow flow make sense — are steps in the right order?
- Do later prompts reference earlier step output correctly?
- If validation found issues, are they concerning?
