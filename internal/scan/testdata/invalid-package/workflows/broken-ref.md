---
type: workflow
id: broken-ref
title: "Broken Reference Workflow"
connections:
  - target: nonexistent-skill
    type: uses
    position: 0
metadata:
  execution:
    - skill: nonexistent-skill
      step_type: generation
---

A workflow that references a skill that does not exist.
