---
type: workflow
id: test-workflow
title: "Test Workflow"
metadata:
  execution:
    - skill: nowhere-skill
      prompt: dep-prompt
      step_type: generation
  stepPrompts:
    nowhere-skill: dep-prompt
---

nowhere-skill is unresolved: not local AND not in hub-shared/test-dep's node list.
