---
type: skill
id: example-generation
title: Example Generation
description: "Produces practical, runnable usage examples in graded tiers from a code analysis, ready to merge into the documentation"
tags: [Production, Documentation, Learning]
connections:
  - target: llm-service
    type: runs_on
metadata:
  estimated_duration: "3 minutes"
  avg_tokens: 2500
  trigger: manual
---

## Example Generation

This skill turns a structural code analysis into practical, runnable usage examples that the documentation stage merges into the final document.

### Core Capability

Given the code analysis, it produces examples in three graded tiers — quick start (copy-paste ready), common patterns (real-world scenarios), and advanced usage (complex integration) — each syntactically valid in the source language and using realistic data rather than placeholder values.

### Output Structure

A markdown block of tiered examples with inline comments and expected output where it aids understanding. The examples feed directly into the documentation-writing stage.
