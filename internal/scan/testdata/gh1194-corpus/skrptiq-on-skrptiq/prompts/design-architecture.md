---
type: prompt
id: design-architecture
title: Design Architecture
description: "Produces a structured skrpt design specification from a validated idea"
tags: [Production, Builder, Stage-2]
inputs:
  validated_idea:
    label: "Validated Idea"
    description: "The output from Stage 1 (Idea Validation). Paste the validated idea summary — it should contain the restated goal, feasibility verdict, node inventory, pipeline pattern, and complexity assessment."
    example: "Restated goal: This skrpt will take a research paper... Feasibility: Feasible. Nodes: 4 skills, 4 prompts, 1 workflow. Pattern: Linear with gate. Complexity: Medium."
    required: true
    type: longtext
connections:
  - target: architecture-design
    type: derived_from
metadata:
  output_format: json
  prompt_type: generation
---

## Purpose

Drives the architecture design step. Takes the validated idea summary from Stage 1 and produces a structured design specification that Stage 3's generator can consume directly.

**Context technique:** This prompt expects a summarized validated idea, not the full Stage 1 conversation transcript. The user (or app) provides the structured analysis section — restated goal, node inventory, pipeline pattern, complexity assessment.

## Prompt

You are a skrpt architect. You have a validated idea and must produce a complete, structured design specification. Your output will be consumed directly by a code generator — it must be precise, complete, and machine-readable.

You have access to the Skrpt Authoring Reference (for node types, connection types, variable system) and Skrpt Patterns Reference (for pipeline architectures).

### Validated Idea

{{input.validated_idea}}

---

Produce a structured design specification covering:

### 1. Nodes

For every node in the skrpt, define:

```json
{
  "type": "skill | prompt | workflow | service | source",
  "id": "kebab-case-id",
  "title": "Title Case Title",
  "description": "What this node does",
  "tags": ["Production", "..."],
  "connections": [
    { "target": "target-id", "type": "uses | derived_from | runs_on | references" }
  ],
  "metadata": {},
  "is_gate": false,
  "purpose": "One sentence: why this node exists in the pipeline"
}
```

Include every node: skills, prompts, workflow, services, sources. Prompts must have exactly one `derived_from` connection. Skills must have `runs_on: llm-service`.

### 2. Execution Plan

Define the pipeline:

```json
{
  "execution": [
    {
      "skill": "skill-id",
      "prompt": "prompt-id",
      "step_type": "generation | content | review | synthesis | validation",
      "context": {}
    }
  ]
}
```

Order matters — execution array order = data flow. Each step should have the inputs it needs from prior steps or user input.

### 3. Loop Configuration (if applicable)

```json
{
  "loops": [
    {
      "id": "loop-id",
      "mode": "until_pass | for_each",
      "steps": ["step-1", "step-2"],
      "verifier": "step-2",
      "maxIterations": 3,
      "freshContextPerIteration": true
    }
  ]
}
```

### 4. Input/Output Contract

**Inputs** — what the user provides:
```json
{
  "inputs": {
    "field_name": {
      "label": "Human Label",
      "description": "What to provide",
      "example": "Example value",
      "required": true,
      "type": "text | longtext"
    }
  }
}
```

**Output** — what the workflow produces:
```json
{
  "output_step": "skill-id",
  "output_description": "What the user gets"
}
```

### 5. Manifest

```json
{
  "name": "kebab-case-name",
  "display_name": "Human Name",
  "description": "What it does",
  "version": "1.0.0",
  "category": "developer | writing | research | shared | ...",
  "tags": ["tag1", "tag2"],
  "requires": {
    "services": ["llm-service"],
    "permissions": [],
    "data_handling": []
  },
  "contents": {
    "skills": 0,
    "prompts": 0,
    "workflows": 1,
    "services": 1,
    "sources": 0,
    "documents": 0,
    "assets": 0,
    "total": 0
  }
}
```

**The `contents` counts MUST match the actual nodes you defined in section 1.**

### 6. Tags

```json
{
  "tags": [
    { "id": "tag-id", "title": "Tag Title", "description": "What it means" }
  ]
}
```

## Formatting Rules

- Output the entire design as a single JSON document
- Use British English in all descriptions and titles
- Every ID must be lowercase with hyphens only
- Every prompt must have exactly one `derived_from` connection to its skill
- Every skill must have a `runs_on` connection to `llm-service`
- Gate skills must have `"is_gate": true` and `metadata: { gate: true }`
- Contents counts must be accurate
- Ground every design choice in the platform's actual capabilities — do not invent features
