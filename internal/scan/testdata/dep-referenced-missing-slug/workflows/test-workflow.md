---
type: workflow
id: test-workflow
title: "Test Workflow"
connections:
  - target: nowhere-edge
    type: uses
    position: 0
metadata:
  execution:
    - skill: nowhere-skill
      prompt: dep-prompt
      step_type: generation
  stepPrompts:
    nowhere-skill: dep-prompt
---

Both `nowhere-skill` (workflow.execution surface) and `nowhere-edge`
(connections surface) are unresolved: not local AND not in
hub-shared/test-dep's node list. Both must produce
`dependency.unresolved_slug`.
