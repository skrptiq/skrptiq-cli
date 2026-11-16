---
type: prompt
id: generate-examples
title: Generate Examples
description: "Creates practical, runnable usage examples showing common patterns and workflows"
tags: [Production, Documentation, Learning]
context_params:
  code_analysis:
    label: "Code Analysis"
    description: "Structural code analysis — the evidence base for the examples."
    required: false
connections:
  - target: code-analysis
    type: derived_from
  - target: documentation-writing
    type: uses
metadata:
  output_format: markdown
  prompt_type: task
---

## Purpose

Produces practical usage examples that demonstrate how to use the code in real scenarios. Examples are the most-referenced section of any documentation — this step gives them dedicated attention rather than treating them as an afterthought.

## Prompt

You are a developer advocate writing usage examples for technical documentation. Using the code analysis below, create a set of practical, runnable examples that the documentation stage will merge into the final document.

- **Code analysis:** {{step.context.code_analysis}}

### Example Requirements

Produce examples in three tiers:

#### 1. Quick Start (1 example)
The simplest possible usage that demonstrates the code works. A developer should be able to copy-paste this and see a result in under a minute. Include:
- Import/require statement
- Minimal setup
- One function call with output
- Expected output shown in a comment

#### 2. Common Patterns (2-4 examples)
Real-world usage patterns covering the most frequent use cases. Each example should:
- Have a descriptive title explaining the scenario
- Include a brief comment explaining what the example demonstrates
- Show complete, runnable code (not fragments)
- Handle errors appropriately
- Use realistic data (not `foo`, `bar`, `test123`)

#### 3. Advanced Usage (1-2 examples)
Complex scenarios that demonstrate deeper capabilities:
- Combining multiple functions together
- Configuration options and customization
- Performance-sensitive patterns
- Integration with common libraries or frameworks

### Writing Rules

- Every example must be syntactically valid and runnable as written
- Use the same programming language as the source code
- Include inline comments explaining non-obvious steps
- Show expected output where it aids understanding
- Use realistic variable names and data that reflect actual use cases
- If the code requires setup (API keys, database connections), show it explicitly and note it as a prerequisite
- Do not wrap examples in unnecessary abstractions — show the direct usage
- If an example could fail (network call, file read), show the error handling
- Progress from simple to complex — each example should build confidence

### Anti-Patterns to Avoid

- Trivial examples that just restate the function signature (`add(1, 2)` for a math library)
- Examples that require undocumented context to understand
- Mixing multiple concepts in one example without explanation
- Using `any` types or ignoring type safety in typed languages
- Omitting imports or setup that the example depends on
