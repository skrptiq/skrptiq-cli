---
type: skill
id: architecture-design
title: Architecture Design
description: "Produces a complete skrpt architecture from a validated idea: node graph, execution plan, connections, and input/output contract"
tags: [Production, Builder, Stage-2]
connections:
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
  - target: skrpt-patterns-reference
    type: references
metadata:
  complexity: high
  avg_tokens: 3000
---

## Capability

Takes a validated idea (from Stage 1) and produces a complete, structured design specification. The output is a machine-readable blueprint that Stage 3's generator can consume directly — not prose, but a structured document with every node defined.

## What It Does

1. **Node inventory** — defines every node: type, ID, title, description, connections, and metadata
2. **Execution plan** — orders the nodes into a pipeline with step types (generation, review, synthesis, validation, content)
3. **Connection mapping** — specifies every edge: `uses`, `derived_from`, `runs_on`, `references`
4. **Input/output contract** — defines what the user provides (`{{input.*}}` variables with labels, descriptions, examples) and what the workflow produces
5. **Loop configuration** — if the pipeline needs iteration, defines the loop: mode, steps, verifier, max iterations
6. **Gate placement** — identifies where human review points belong and marks those skills as gates
7. **Manifest values** — specifies name, display_name, description, version, category, tags, requires, and accurate contents counts

## Output

Structured design specification in JSON or YAML format containing:
- `nodes[]` — every node with full metadata
- `execution[]` — the pipeline definition
- `loops[]` — any loop configurations
- `manifest` — package metadata
- `tags[]` — tag definitions
- `input_contract` — user-facing inputs with metadata
- `output_contract` — what the workflow produces

This output is designed to be consumed directly by the Stage 3 generator with no interpretation needed.

## Limitations

The design is as good as the validated idea. Vague ideas produce vague designs. The architecture step does not invent requirements — it structures what the validation step approved.
