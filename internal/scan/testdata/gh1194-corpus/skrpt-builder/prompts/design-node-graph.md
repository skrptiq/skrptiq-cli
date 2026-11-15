---
type: prompt
id: design-node-graph
title: Design Node Graph
description: "Produces a detailed node inventory and execution plan from the approved requirements"
tags: [Production]
connections:
  - target: node-graph-design
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Takes the approved requirements and user feedback, and produces a complete blueprint for every node in the skrpt.

## Prompt

You are a skrpt architect. Using the skrpt authoring reference and patterns reference, design the complete node graph for this skrpt.

### Approved Requirements and User Feedback

{{steps.previous.output}}

### Design Steps

1. **Name the skrpt** — derive a kebab-case slug from the goal (e.g. "pr-review-pipeline")
2. **List every node** with: type, id (kebab-case), title (Title Case), description, connections array
3. **For each skill:** decide step_type (generation/content/review/synthesis/validation), whether it's a gate, and what service it runs on
4. **For each prompt:** set derived_from to its skill, declare inputs if it's the first step, reference upstream output via `{{steps.previous.output}}` or `{{steps.Step Title.output}}` for later steps
5. **Design the execution block** — step order, any parallel blocks, any loops
6. **If loops needed:** define the loop in workflow frontmatter (NOT in skrptiq.yaml) with id, mode, steps, verifier (for until_pass), maxIterations
7. **Calculate contents counts** — count each node type, calculate total
8. **Set requires block** — list services, permissions (network domains if any), data_handling flags

### Rules

- Every non-utility skill must have exactly one prompt with `derived_from` pointing to it
- Prompt IDs should follow the pattern: `skill-id-prompt` or descriptive kebab-case
- The workflow must have `uses` connections to every skill and `runs_on` to every service
- `output_step` should be the last meaningful skill
- `composite_steps` should include all skills that contribute to the final output
- Loops go in **workflow frontmatter only**, never in skrptiq.yaml

### Output Format

```
skrpt_name: "SLUG"
display_name: "DISPLAY NAME"
description: "DESCRIPTION"
category: "CATEGORY"
tags: ["tag1", "tag2"]

nodes:
  - type: skill
    id: "SKILL_ID"
    title: "SKILL TITLE"
    description: "DESCRIPTION"
    step_type: "generation"
    is_gate: false
    connections:
      - target: llm-service
        type: runs_on

  - type: prompt
    id: "PROMPT_ID"
    title: "PROMPT TITLE"
    description: "DESCRIPTION"
    connections:
      - target: SKILL_ID
        type: derived_from
    inputs:
      - name: "var_name"
        label: "Label"
        description: "Description"
        required: true

  - type: workflow
    id: "WORKFLOW_ID"
    title: "WORKFLOW TITLE"
    connections: [...]
    output_step: "FINAL_SKILL"
    composite_steps: [...]
    execution: [...]
    loops: [...]  # if needed

  - type: service
    id: "llm-service"

requires:
  services: ["llm-service"]
  permissions: []
  data_handling: []

contents:
  skills: N
  prompts: N
  workflows: 1
  services: 1
  sources: 0
  documents: 0
  assets: 0
  total: N
```

Be precise with IDs and connections — the generation step will use this blueprint verbatim.
