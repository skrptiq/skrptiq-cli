---
type: asset
id: node-templates-reference
title: Node Templates Reference
description: "Fill-in-the-blank skeleton templates for every node type and the manifest"
tags: [Reference, Authoring]
connections: []
metadata:
  format: yaml
---

## Manifest Template (skrptiq.yaml)

```yaml
name: "SLUG"
display_name: "DISPLAY NAME"
description: "DESCRIPTION"
version: "1.0.0"
engine: ">=1.0.0"

author:
  name: "AUTHOR"
  url: "AUTHOR_URL"

licence: "MIT"
category: "CATEGORY"
tags:
  - "TAG1"
  - "TAG2"

requires:
  services:
    - "llm-service"
  permissions: []
  data_handling: []

contents:
  skills: COUNT
  prompts: COUNT
  workflows: 1
  services: 1
  sources: COUNT
  documents: COUNT
  assets: COUNT
  total: TOTAL
```

## Tags Template (tags.yaml)

```yaml
- id: "TAG_ID"
  title: "TAG TITLE"
  description: "TAG DESCRIPTION"
```

## Skill Template

```yaml
---
type: skill
id: SKILL_ID
title: SKILL TITLE
description: "DESCRIPTION"
tags: [Production]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

WHAT THIS SKILL DOES.

## When to Use

WHEN TO USE THIS SKILL.

## What It Does

1. STEP ONE
2. STEP TWO
3. STEP THREE

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.VAR}}` | Yes | DESCRIPTION | EXAMPLE |

## Outputs

WHAT THIS SKILL PRODUCES.
```

## Gate Skill Template

```yaml
---
type: skill
id: GATE_SKILL_ID
title: GATE TITLE
description: "GATE DESCRIPTION"
tags: [Production, Gate]
connections:
  - target: llm-service
    type: runs_on
metadata:
  gate: true
---

## Capability

Pauses the workflow for user review. The user's response becomes this step's output.

## What Happens

1. Execution pauses with status `awaiting_input`
2. The user sees the output from the previous step
3. The user responds with approval or feedback
4. The response becomes this step's output

## Review Guidance

WHAT THE USER SHOULD CHECK.
```

## Prompt Template

```yaml
---
type: prompt
id: PROMPT_ID
title: PROMPT TITLE
description: "DESCRIPTION"
tags: [Production]
inputs:
  INPUT_NAME:
    label: "LABEL"
    description: "DESCRIPTION"
    example: "EXAMPLE"
    required: true
    type: text
connections:
  - target: SKILL_ID
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

WHAT THIS PROMPT DOES.

## Prompt

ROLE INSTRUCTION.

### Steps

1. STEP ONE
2. STEP TWO
3. STEP THREE

### Input

- **LABEL:** {{input.INPUT_NAME}}

### Output Format

DESCRIBE EXPECTED OUTPUT STRUCTURE.
```

## Later-Stage Prompt Template

For prompts that consume output from a previous step (step 2+):

```yaml
---
type: prompt
id: PROMPT_ID
title: PROMPT TITLE
description: "DESCRIPTION"
tags: [Production]
connections:
  - target: SKILL_ID
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

WHAT THIS PROMPT DOES WITH THE PREVIOUS OUTPUT.

## Prompt

ROLE INSTRUCTION.

### Input

Previous step output:

{{steps.previous.output}}

### Steps

1. STEP ONE
2. STEP TWO

### Output Format

DESCRIBE EXPECTED OUTPUT STRUCTURE.
```

## Gate Prompt Template (Loop Verifier)

```yaml
---
type: prompt
id: GATE_PROMPT_ID
title: GATE TITLE
description: "DESCRIPTION"
tags: [Production, Gate]
connections:
  - target: GATE_SKILL_ID
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Pauses for user review and collects approval or feedback.

## Prompt

Review the output below. If it meets your requirements, type **approved**. Otherwise, describe what needs to change.

### Output to Review

{{steps.previous.output}}

### What to Check

- CHECK ITEM ONE
- CHECK ITEM TWO
- CHECK ITEM THREE
```

## Workflow Template

```yaml
---
type: workflow
id: WORKFLOW_ID
title: WORKFLOW TITLE
description: "DESCRIPTION"
tags: [Production]
connections:
  - target: SKILL_ONE
    type: uses
  - target: SKILL_TWO
    type: uses
  - target: llm-service
    type: runs_on
metadata:
  estimated_duration: "DURATION"
  trigger: manual
output_step: "FINAL_SKILL"
composite_steps:
  - "SKILL_ONE"
  - "SKILL_TWO"
  - "FINAL_SKILL"
execution:
  - skill: "SKILL_ONE"
    prompt: "PROMPT_ONE"
    step_type: "generation"
  - skill: "SKILL_TWO"
    prompt: "PROMPT_TWO"
    step_type: "synthesis"
  - skill: "FINAL_SKILL"
    prompt: "FINAL_PROMPT"
    step_type: "content"
---

## Overview

WHAT THIS WORKFLOW DOES.

## Pipeline Stages

### Stage 1: TITLE

DESCRIPTION.

### Stage 2: TITLE

DESCRIPTION.

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|

## Outputs

| Name | Description |
|------|-------------|
```

## Service Template (LLM)

```yaml
---
type: service
id: llm-service
title: LLM Service
description: "Language model service for analysis, synthesis, and document generation"
tags: [Production]
connections: []
metadata:
  serviceType: llm
  auth_type: api_key
---
```

## Source Template

```yaml
---
type: source
id: SOURCE_ID
title: SOURCE TITLE
description: "DESCRIPTION"
tags: [Reference]
connections: []
---

REFERENCE CONTENT HERE.
```
