---
type: workflow
id: example-workflow
title: "Example Workflow"
description: "A valid test workflow"
connections:
  - target: example-skill
    type: uses
    position: 0
metadata:
  execution:
    - skill: example-skill
      prompt: example-prompt
      step_type: generation
  stepPrompts:
    example-skill: example-prompt
---

A simple workflow that runs one skill.
