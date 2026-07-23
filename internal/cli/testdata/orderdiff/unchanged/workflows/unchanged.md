---
type: workflow
id: unchanged-fixture
title: "Unchanged Fixture"
description: "Beta consumes Alpha's output, and Alpha is authored first — derivation's edge preserves the existing order."
execution:
  - action: "mcp:test:alpha"
    label: "Alpha"
    gate: false
    output: { name: "alpha_out", type: "text" }
  - action: "mcp:test:beta"
    label: "Beta"
    gate: false
    context:
      data: "{{steps.Alpha.output}}"
    output: { name: "beta_out", type: "text" }
---

Test workflow: Beta names Alpha; Alpha already runs first, so the order is unchanged.
