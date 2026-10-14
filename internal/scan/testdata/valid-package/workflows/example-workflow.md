---
type: workflow
id: example-workflow
title: "Example Workflow"
description: "A valid test workflow"
connections:
  - target: example-skill
    type: uses
    position: 0
  - target: summary-skill
    type: uses
    position: 1
metadata:
  execution:
    - skill: example-skill
      prompt: example-prompt
      step_type: generation
    - skill: summary-skill
      prompt: spaced-ref-prompt
      step_type: generation
  stepPrompts:
    example-skill: example-prompt
    summary-skill: spaced-ref-prompt
---

A simple workflow that runs two skills, the second summarising the first.
