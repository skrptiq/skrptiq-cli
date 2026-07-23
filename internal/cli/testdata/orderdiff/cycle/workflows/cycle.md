---
type: workflow
id: cycle-fixture
title: "Cycle Fixture"
description: "Ping consumes Pong's output and Pong consumes Ping's — derivation makes the graph cyclic, which must fail loud (#282)."
execution:
  - action: "mcp:test:ping"
    label: "Ping"
    gate: false
    context:
      data: "{{steps.Pong.output}}"
    output: { name: "ping_out", type: "text" }
  - action: "mcp:test:pong"
    label: "Pong"
    gate: false
    context:
      data: "{{steps.Ping.output}}"
    output: { name: "pong_out", type: "text" }
---

Test workflow: Ping and Pong reference each other, forming a cycle.
