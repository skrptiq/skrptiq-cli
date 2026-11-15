---
type: skill
id: design-gate
title: Design Gate
description: "Human gate — pauses for the user to review the architecture design and approve or correct before build begins"
tags: [Production, Gate, Stage-2]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Pauses the workflow and presents the structured design specification to the user for review. The user either approves the architecture or provides corrections before the build stage begins.

## What Happens

1. Execution pauses with status `awaiting_input`
2. The user sees the structured design: node inventory, execution plan, connections, input/output contract, loop config
3. The user responds with approval or feedback
4. The response becomes this step's output — passed to Stage 3 (Skrpt Build) as the design specification

## Review Guidance

- Does the node inventory match what you expected from the validated idea?
- Is the execution order logical — does each step have the inputs it needs?
- Are the gate placements right — do you need approval points where they are?
- Is the input/output contract clear — will users understand what to provide?
- If there's a loop, is the verifier correct and is the max iteration count reasonable?
- Are the manifest values (name, description, category, tags) accurate?

## How to Respond

- **Approve:** Type "approved" — the design passes to Stage 3 as-is. The generator will use this specification to produce all files.
- **Correct:** Provide specific feedback — "add a gate between steps 2 and 3", "rename the skill to X", "remove the loop, make it linear". Be specific — this is the last review before generation.
