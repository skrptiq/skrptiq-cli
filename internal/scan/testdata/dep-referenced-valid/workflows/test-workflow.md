---
type: workflow
id: test-workflow
title: "Test Workflow"
description: "Exercises dep-provided refs in workflow.execution"
metadata:
  execution:
    - skill: dep-skill
      prompt: dep-prompt
      step_type: generation
  stepPrompts:
    dep-skill: dep-prompt
---

dep-skill and dep-prompt resolve via the declared hub-shared/test-dep dep.
