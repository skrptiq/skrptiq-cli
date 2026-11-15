---
type: skill
id: final-assembly
title: Final Assembly
description: "Assembles the approved skrpt files into the final structured output"
tags: [Production]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Takes the approved generated files and produces the final output: a complete, structured skrpt package ready to be saved as a workspace.

## When to Use

As the last step in the builder, after the user has approved the generated skrpt.

## What It Does

1. **Directory listing** — shows the complete file structure
2. **All files** — each file as a labeled, copyable code block
3. **Structured data** — JSON output the app can use to create the workspace
4. **Quick start** — explains what was built, how to save it, how to run it

## Outputs

Complete skrpt package: directory structure, all files as code blocks, structured JSON for the app, and setup instructions.
