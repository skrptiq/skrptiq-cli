---
type: workflow
id: test-workflow
title: "Test Workflow"
metadata:
  execution:
    - skill: dep-skill
      prompt: dep-prompt
      step_type: generation
  stepPrompts:
    dep-skill: dep-prompt
---

Checksum mismatch must trigger dependency.checksum_mismatch.
