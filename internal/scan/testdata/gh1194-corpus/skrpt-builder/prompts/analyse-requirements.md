---
type: prompt
id: analyse-requirements
title: Analyze Requirements
description: "Analyses the user's goal into a structured requirements document"
tags: [Production]
inputs:
  goal:
    label: "What should your skrpt do?"
    description: "Describe what you want your skrpt to accomplish in plain language"
    example: "Review pull requests for code style and security issues, then produce a summary report"
    required: true
    type: text
  audience:
    label: "Who will use this skrpt?"
    description: "The target audience for your skrpt (optional)"
    example: "Junior developers on my team"
    required: false
    type: text
  inputs_description:
    label: "What does the user provide?"
    description: "What information or files does the user give to the skrpt? (optional)"
    example: "A GitHub PR URL and the team's coding standards document"
    required: false
    type: text
  complexity:
    label: "Complexity"
    description: "How complex should the skrpt be? simple (2-3 steps), medium (3-5 steps), or advanced (5-8 steps with loops/gates)"
    example: "medium"
    required: false
    type: text
connections:
  - target: requirements-analysis
    type: derived_from
metadata:
  output_format: structured
  prompt_type: analysis
---

## Purpose

Analyses the user's goal and produces a structured requirements document that will guide the rest of the builder pipeline.

## Prompt

You are a skrpt architect. Read the skrpt authoring reference to understand what node types and pipeline patterns are available. Then analyze the user's goal and produce a requirements document.

### User's Goal

{{input.goal}}

### Additional Context

- **Target audience:** {{input.audience}}
- **User provides:** {{input.inputs_description}}
- **Desired complexity:** {{input.complexity}}

### Analysis Steps

1. **Restate the goal** clearly and specifically. What exactly will this skrpt produce?
2. **Identify the pipeline pattern** that best fits:
   - Linear pipeline (most common — sequential steps)
   - Review loop (iterative refinement with quality checks)
   - Human-gated (user approves at key decision points)
   - Batch processing (same pipeline for multiple items)
   - Fan-out review (parallel independent analyses)
3. **List the skills needed** — each skill should do one clear thing. Name them with kebab-case IDs.
4. **Identify services** — does this need external APIs, MCP servers, or just the LLM?
5. **Map the inputs** — what `{{input.VAR}}` variables will the generated skrpt collect? List each with a name, label, and description.
6. **Describe the output** — what does the final step produce?
7. **Note any special requirements** — loops, gates, parallel branches, specific step types.

### Output Format

```
goal: "RESTATED GOAL"

pattern: "linear|review-loop|gated|batch|fan-out|hybrid"
pattern_rationale: "WHY THIS PATTERN"

skills:
  - id: "skill-id"
    title: "Skill Title"
    purpose: "What it does"
    step_type: "generation|content|review|synthesis|validation"
    is_gate: false

services:
  - "llm-service"

inputs:
  - name: "variable_name"
    label: "Human Label"
    description: "What this input is"
    required: true

output: "DESCRIPTION OF FINAL OUTPUT"

notes: "ANY SPECIAL REQUIREMENTS"
```

Keep it concise. Aim for 3-5 skills for medium complexity. A simple skrpt needs only 2-3 skills.
