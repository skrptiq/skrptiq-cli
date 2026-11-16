---
type: prompt
id: analyse-code
title: Analyze Code
description: "Extracts structural information from source code for documentation generation"
tags: [Production, Code, Documentation]
inputs:
  language:
    label: "Language"
    description: "The language"
    example: "French"
    required: true
    type: text
  source_code:
    label: "Source Code"
    description: "The code to review or analyze"
    example: "[Paste source code here]"
    required: true
    type: text
connections:
  - target: code-analysis
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: core
---

## Purpose

Drives the code analysis skill by extracting every documentable element from the source code. The output of this step feeds directly into documentation generation.

## Prompt

You are a senior software engineer performing a documentation audit. Analyze the following source code and produce a complete structural inventory.

### Source Code

**Language:** {{input.language}}

```
{{input.source_code}}
```

### Analysis Required

For each module or file in the code, extract:

1. **Module overview** — what this code does in 2-3 sentences, its role in a broader system if apparent
2. **Public API surface** — every exported function, class, constant, or type that external code can use
3. **Functions and methods** — for each:
   - Name and full signature
   - Parameters with types and descriptions (infer from names, usage, and context if types are absent)
   - Return type and what it represents
   - Side effects (file I/O, network calls, state mutation, logging)
   - Exceptions or errors it can throw/return
   - Whether it is public, private, or internal
4. **Classes and types** — for each:
   - Purpose and single responsibility
   - Constructor parameters
   - Properties with types and visibility
   - Methods (reference the function analysis above)
   - Inheritance, interface implementations, or mixin relationships
5. **Constants and configuration** — named constants, default values, environment variables read
6. **Dependencies** — imports and what they are used for; distinguish standard library, third-party, and internal
7. **Patterns and conventions** — error handling approach, naming conventions, architectural patterns (factory, observer, middleware, etc.)
8. **Complexity notes** — any functions over 50 lines, deeply nested logic, or non-obvious control flow

### Output Format

Structure your analysis as a markdown document with clear sections. Use tables for parameter lists and function signatures. Flag anything ambiguous with `[UNCLEAR]` — do not guess when the code is genuinely ambiguous.

If the code is incomplete (a snippet rather than a full file), note what context is missing and analyze what you can see.
