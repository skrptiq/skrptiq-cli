---
type: workflow
id: for-each-workflow
title: "For Each Workflow"
description: "for_each loop with body referencing a dep-provided prompt whose Content has {{loop.item}}"
connections:
  - target: dep-skill
    type: uses
    position: 0
metadata:
  execution:
    - skill: dep-skill
      prompt: dep-prompt
      step_type: generation
  loops:
    - id: "process-items"
      mode: "for_each"
      inputExpression: "{{$.items}}"
      steps:
        - "dep-skill"
      maxIterations: 5
---

The loop body's prompt (dep-prompt) lives in hub-shared/test-dep and
contains `{{loop.item}}`. Without dep-aware validation,
workflow.loop_for_each_no_loop_item_usage would fire. With GH#650
dep-aware validation, the engine sees the dep summary's Content and
the check passes.
