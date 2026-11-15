---
type: prompt
id: assemble-skrpt
title: Assemble Skrpt
description: "Produces the final structured output from the approved skrpt files"
tags: [Production]
connections:
  - target: final-assembly
    type: derived_from
metadata:
  output_format: structured
  prompt_type: task
---

## Purpose

Takes the approved skrpt and produces the final deliverable: a complete package with all files ready to save.

## Prompt

You are assembling the final skrpt package. The user has approved the generated files. Produce the complete output.

### Approved Skrpt

{{steps.previous.output}}

### Assembly Steps

1. **Directory structure** — show the complete file tree
2. **All files** — output every file with its path and complete contents
3. **Structured JSON** — produce a machine-readable manifest the app can use to create the workspace
4. **Quick start guide** — explain what was built and how to use it

### Output Format

#### Directory Structure

```
skrpt-name/
├── skrptiq.yaml
├── tags.yaml
├── services/
│   └── llm-service.md
├── skills/
│   ├── ...
├── prompts/
│   ├── ...
└── workflows/
    └── ...
```

#### Files

(Each file as a fenced code block with its path as heading)

#### Save Data

```json
{
  "skrpt_name": "skrpt-name",
  "display_name": "Skrpt Name",
  "files": [
    { "path": "skrptiq.yaml", "content": "..." },
    { "path": "skills/skill-name.md", "content": "..." }
  ]
}
```

#### Quick Start

1. **Save** — use the "Save as Skrpt" button to create your workspace, or copy each file manually
2. **Configure** — if your skrpt uses external services, add the required connections in Settings > Connections
3. **Run** — open the workflow and click Run. Fill in the input form and go.

Include any service-specific setup notes (e.g. "add your GitHub API key" if the skrpt uses github-mcp).
