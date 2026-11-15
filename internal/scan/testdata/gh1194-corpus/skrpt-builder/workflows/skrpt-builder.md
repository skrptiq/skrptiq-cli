---
type: workflow
id: skrpt-builder
title: Skrpt Builder
description: "Design and generate a complete skrpt from a plain-language description of what you want it to do"
tags: [Production, Builder]
connections:
  - target: requirements-analysis
    type: uses
  - target: architecture-review
    type: uses
  - target: node-graph-design
    type: uses
  - target: node-generation
    type: uses
  - target: structure-validation
    type: uses
  - target: skrpt-review
    type: uses
  - target: final-assembly
    type: uses
  - target: llm-service
    type: runs_on
  - target: skrpt-authoring-reference
    type: references
  - target: skrpt-patterns-reference
    type: references
  - target: node-templates-reference
    type: references
metadata:
  estimated_duration: "5-15 minutes"
  trigger: manual
loops:
  - id: "build-review"
    mode: "until_pass"
    steps:
      - "node-generation"
      - "structure-validation"
      - "skrpt-review"
    verifier: "skrpt-review"
    maxIterations: 3
    freshContextPerIteration: true
output_step: "final-assembly"
composite_steps:
  - "requirements-analysis"
  - "node-graph-design"
  - "node-generation"
  - "structure-validation"
  - "final-assembly"
execution:
  - skill: "requirements-analysis"
    prompt: "analyse-requirements"
    step_type: "generation"
    output: { name: "requirements", type: "text" }
  - skill: "architecture-review"
    prompt: "review-architecture"
    step_type: "validation"
    gate: true
    output: { name: "architecture_review", type: "text" }
  - skill: "node-graph-design"
    prompt: "design-node-graph"
    step_type: "generation"
    output: { name: "node_graph", type: "text" }
  - skill: "node-generation"
    prompt: "generate-nodes"
    step_type: "generation"
    output: { name: "nodes", type: "text" }
  - skill: "structure-validation"
    prompt: "validate-structure"
    step_type: "validation"
    output: { name: "validation_result", type: "decision" }
  - skill: "skrpt-review"
    prompt: "review-skrpt"
    step_type: "validation"
    gate: true
    output: { name: "review_feedback", type: "text" }
  - skill: "final-assembly"
    prompt: "assemble-skrpt"
    step_type: "synthesis"
    output: { name: "assembled_skrpt", type: "text" }
---

## Overview

The Skrpt Builder guides you through creating a complete skrpt from a plain-language description. Tell it what you want your skrpt to do, review the proposed architecture, and it generates every file — skills, prompts, workflow, manifest — ready to save as a workspace.

This is the recommended way to create your first skrpt. The builder understands all node types, connection patterns, and pipeline architectures, so it produces structurally valid output that follows best practices.

## Pipeline Stages

### Stage 1: Requirements Analysis

**Input:** Your goal, audience, and complexity preference.

The builder analyses your description and proposes an architecture: which skills you need, what pipeline pattern fits (linear, loop, gated, parallel), what inputs your skrpt will collect, and what it will produce.

### Stage 2: Architecture Review (Gate)

Execution pauses. You review the proposed architecture:
- Is the flow right?
- Are the skills appropriate?
- Should anything be added or removed?

Approve to continue, or provide corrections.

### Stage 3: Node Graph Design

Taking your approval and any corrections, the builder produces a detailed blueprint: every node with its ID, connections, and the complete execution plan.

### Stage 4-5-6: Build-Review Loop

The builder enters an iterative loop (up to 3 iterations):

1. **Node Generation** — generates all file contents from the blueprint
2. **Structure Validation** — checks the output against format rules
3. **User Review (Gate)** — you review the files and validation results

Approve to exit the loop, or provide feedback for revision.

### Stage 7: Final Assembly

Produces the final package: directory structure, all files as copyable code blocks, structured JSON for the app's "Save as Skrpt" feature, and a quick start guide.

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| Goal | Yes | What should your skrpt do? | "Review PRs for code style and security issues" |
| Audience | No | Who will use this skrpt? | "Junior developers on my team" |
| User provides | No | What information does the user give? | "A GitHub PR URL and coding standards" |
| Complexity | No | simple, medium, or advanced | "medium" |

## Outputs

| Name | Description |
|------|-------------|
| Skrpt package | Complete file set: manifest, skills, prompts, workflow, services, and any sources/documents/assets |
| Save data | Structured JSON the app can use to create the workspace |
| Quick start | Setup and usage instructions |

## Tips

- **Start simple.** Choose "simple" complexity for your first skrpt. You can always add steps later.
- **Be specific about your goal.** "Summarize meeting notes into action items and decisions" is better than "help with meetings".
- **Describe the user's input.** If your skrpt needs a document, URL, or data, say so — it helps the builder design the right input form.
- **Review the prompts carefully.** The prompts are where the real work happens. Make sure they capture your intent.
