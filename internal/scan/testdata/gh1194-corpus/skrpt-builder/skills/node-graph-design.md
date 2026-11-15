---
type: skill
id: node-graph-design
title: Node Graph Design
description: "Produces a detailed node inventory with IDs, connections, execution plan, and manifest values"
tags: [Production]
connections:
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
  - target: skrpt-patterns-reference
    type: references
---

## Capability

Takes the approved requirements and produces a detailed blueprint: every node with its type, ID, title, connections, and the complete execution plan.

## When to Use

After the user has approved the architecture, before file generation begins.

## What It Does

1. **Assign IDs** — kebab-case IDs for every node
2. **Map connections** — derived_from, uses, runs_on, references for each node
3. **Design the execution block** — step order, parallel blocks, loop config
4. **Calculate manifest values** — name, requires block, contents counts
5. **Inventory input variables** — every `{{input.*}}` and `{{steps.X.output}}` reference

## Outputs

Structured node inventory: complete list of nodes with types, IDs, titles, connections, the workflow execution block, and manifest field values.
