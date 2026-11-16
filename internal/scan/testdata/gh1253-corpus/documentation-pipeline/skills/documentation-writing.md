---
type: skill
id: documentation-writing
title: Documentation Writing
description: "Produces technical documentation from code analysis, covering API references, guides, and module overviews"
tags: [Production, Code, Documentation, Writing]
connections:
  - target: llm-service
    type: runs_on
  - target: documentation-standards
    type: references
---

## Capability

Transforms structured code analysis into clear, detailed technical documentation. Produces API references, module overviews, and usage guides tailored to the target audience. Adapts depth and terminology based on whether readers are library consumers, contributors, or end users.

## When to Use

- Generating initial documentation for undocumented code
- Updating docs after significant refactoring
- Creating API reference documentation for library consumers
- Writing internal developer guides for onboarding

## Inputs

Code analysis output, target audience, documentation type (API reference, README, guide, inline docs)

## Outputs

Technical documentation: module/function descriptions, parameter tables with types, return value documentation, edge cases, error conditions, and cross-references between related components
