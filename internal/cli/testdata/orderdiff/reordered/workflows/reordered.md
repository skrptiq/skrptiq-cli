---
type: workflow
id: reordered-fixture
title: "Reordered Fixture"
description: "First step consumes Second's output; derivation must move Second before First. First also uses {{steps.previous.output}} to exercise the positional-ref hazard flag."
execution:
  - action: "mcp:test:first"
    label: "First"
    gate: false
    context:
      data: "{{steps.Second.output}}"
      note: "{{steps.previous.output}}"
    output: { name: "first_out", type: "text" }
  - action: "mcp:test:second"
    label: "Second"
    gate: false
    output: { name: "second_out", type: "text" }
---

Test workflow: First names Second, so derivation orders Second before First.
