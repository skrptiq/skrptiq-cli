---
type: prompt
id: review-design
title: Review Design
description: "Presents the architecture design to the user for approval before build begins"
tags: [Production, Gate, Stage-2]
connections:
  - target: design-gate
    type: derived_from
metadata:
  output_format: plain
  prompt_type: gate
---

## Purpose

Gate prompt for Stage 2. Presents the structured design specification and collects the user's approval or corrections before the build stage.

## Prompt

The architecture design is complete. Review the specification below and decide whether to proceed to the build stage.

### Design Specification

{{steps.Architecture Design.output}}

---

### What to Check

1. **Node inventory** — are all the nodes you expected present? Any missing or unnecessary?
2. **Execution order** — does the pipeline flow make sense? Does each step have the inputs it needs?
3. **Connections** — are `derived_from`, `runs_on`, and `uses` edges all correct?
4. **Input contract** — will users understand what to provide? Are the examples helpful?
5. **Output** — is the output step correct? Does the output description match what you expect?
6. **Loop config** — if applicable, is the verifier right? Is the max iteration count reasonable?
7. **Manifest** — are name, description, category, and tags accurate?
8. **Contents counts** — do the counts match the nodes in the design?

### How to Respond

- **To approve:** Type "approved" — the design passes to Stage 3 (Skrpt Build) where files will be generated from this specification
- **To correct:** Provide specific feedback — "add a gate after step 2", "change the category to writing", "the input needs a third field for X". Be precise — this specification is what the generator uses.
- **To stop:** Type "stop" if the design needs rethinking from the idea stage

Your response becomes the input to Stage 3. If you approve, the design specification is passed as-is. If you correct, the corrections are noted alongside the design.
