---
type: prompt
id: write-documentation
title: Write Documentation
description: "Generates detailed technical documentation from code analysis output"
tags: [Production, Code, Documentation]
inputs:
  audience:
    label: "Audience"
    description: "Who this is for"
    example: "Product managers at B2B SaaS companies"
    required: true
    type: text
  doc_type:
    label: "Documentation Type"
    description: "The type of documentation to generate"
    example: "API reference"
    required: true
    type: text
context_params:
  code_analysis:
    label: "Code Analysis"
    description: "Structural code analysis — the source material for the documentation."
    required: false
  usage_examples:
    label: "Usage Examples"
    description: "Tiered usage examples to incorporate into the documentation."
    required: false
connections:
  - target: documentation-writing
    type: derived_from
  - target: documentation-standards
    type: references
metadata:
  output_format: markdown
  prompt_type: task
---

## Purpose

Produces technical documentation from the structural analysis performed in the previous stage. This is the primary documentation generation step.

## Prompt

You are a technical writer producing documentation for a codebase. Your target audience is: **{{input.audience}}**. The documentation type is: **{{input.doc_type}}**.

Using the code analysis below, write complete documentation. Incorporate the usage examples into the relevant sections rather than treating them as an afterthought.

**Code analysis:** {{step.context.code_analysis}}

**Usage examples:** {{step.context.usage_examples}}

### Documentation Requirements

1. **Overview** — what this code does, when to use it, and any prerequisites or dependencies
2. **Installation or setup** — if applicable based on the code (library imports, configuration, environment requirements)
3. **API reference** — for each public function, class, or type:
   - Signature with types
   - Description of what it does (focus on *what* and *why*, not *how* it is implemented)
   - Parameters table: name, type, required/optional, default value, description
   - Return value: type and description
   - Exceptions/errors: when they occur and what they mean
   - One short example showing basic usage
4. **Configuration** — if the code reads environment variables, config files, or accepts options, document every option with its type, default, and effect
5. **Edge cases and gotchas** — non-obvious behavior, common mistakes, performance considerations, thread safety notes
6. **Error handling** — what errors can occur, what they mean, and how callers should handle them

### Writing Rules

- Write for the specified audience — adjust depth and assumed knowledge accordingly
- Lead with the most important information; implementation details follow
- Every public function or method gets at least one inline example
- Use consistent terminology — if the code calls it a "handler", the docs call it a "handler"
- Do not document private or internal implementation details unless they affect the public API
- If something is unclear from the code, say so explicitly rather than inventing behavior
- Keep descriptions concise — one sentence for simple functions, a short paragraph for complex ones
- Use present tense ("Returns the user object" not "Will return the user object")
- Prefer concrete descriptions over abstract ones ("Retries the request up to 3 times" not "Handles retries")

### Audience Guide

Adjust your writing based on the audience:

- **Developers (library consumers):** Focus on how to use the API. Assume programming competence but no knowledge of internals. Every function needs clear inputs, outputs, and examples.
- **Contributors:** Include architectural context, design decisions, and internal patterns. Explain *why* things work the way they do.
- **End users:** Avoid code-level details. Focus on configuration, behavior, and troubleshooting. Use plain language.
- **Mixed/general:** Default to library consumer level. Add a "For Contributors" section at the end with deeper architectural notes.
