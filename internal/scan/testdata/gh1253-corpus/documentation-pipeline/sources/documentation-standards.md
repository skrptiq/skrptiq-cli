---
type: source
id: documentation-standards
title: Documentation Standards
description: "Reference standards for technical documentation format, structure, and content quality"
tags: [Production, Documentation, Quality]
connections: []
metadata:
  last_updated: "2026-03-01"
  applies_to: [javascript, typescript, python, go, rust, java]
---

## Purpose

Defines the standards that all generated documentation must follow. The documentation writing and formatting skills use this as their primary reference for quality and consistency.

## Document Structure

### Required Sections

Every documentation output must include these sections in order:

1. **Title and description** — one-line summary of what the code does
2. **Overview** — 2-5 sentences explaining purpose, when to use it, and prerequisites
3. **Installation/Setup** — how to install or configure (omit if not applicable)
4. **API Reference** — every public symbol documented
5. **Examples** — practical usage, progressing from simple to complex
6. **Edge Cases and Gotchas** — non-obvious behavior, common mistakes

### Optional Sections

Include when relevant:

- **Configuration** — environment variables, options, feature flags
- **Migration Guide** — when documenting changes to existing APIs
- **Troubleshooting** — common errors and their solutions
- **Contributing** — for contributor-facing documentation
- **Changelog** — version history with breaking changes highlighted

## Function and Method Documentation

### Standard Format

```
### `functionName(param1, param2, options?)`

Description of what the function does. One sentence for simple functions,
a short paragraph for complex ones.

**Parameters:**

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| param1 | string | Yes | — | What this parameter controls |
| param2 | number | Yes | — | What this parameter controls |
| options | Object | No | {} | Configuration options |

**Returns:** `ReturnType` — description of what is returned

**Throws:**
- `ErrorType` — when this error occurs and why

**Example:**

(code block with runnable example)
```

### Rules

- Document what the function does, not how it is implemented
- Parameter descriptions should explain purpose, not just restate the name
- Include default values for all optional parameters
- Document every error/exception the function can produce
- Use the function's actual return type, not a generic description

## Code Examples

### Quality Standards

- Every example must be syntactically valid and runnable
- Use fenced code blocks with the correct language identifier
- Include import/require statements — examples must be self-contained
- Use realistic data, not placeholder values (no `foo`, `bar`, `test`)
- Show expected output in comments where it aids understanding
- Handle errors in examples that involve I/O, network, or parsing
- Progress from simple to complex across the examples section

### Example Tiers

| Tier | Purpose | Count |
|------|---------|-------|
| Quick start | Simplest working usage, copy-paste ready | 1 |
| Common patterns | Real-world scenarios covering frequent use cases | 2-4 |
| Advanced | Complex integration, customization, performance | 1-2 |

## Markdown Formatting

### Headings

- H1 (`#`) for the document title — exactly one per document
- H2 (`##`) for major sections (Overview, API Reference, Examples)
- H3 (`###`) for individual functions, classes, or subsections
- H4 (`####`) for sub-items within a function (parameters, examples)
- Never skip heading levels (no H1 to H3 without H2)

### Code

- Use fenced code blocks (triple backtick) with language identifier
- Inline code (single backtick) for function names, parameter names, types, file paths, and terminal commands
- No screenshots of code — always use text

### Tables

- Use pipe tables for parameter documentation
- Align columns with consistent spacing
- Include a header row with separator
- Use `—` (em dash) for "not applicable" cells, not empty cells

### Lists

- Bullet points for unordered items (3+ related items)
- Numbered lists only when sequence matters (steps, priority order)
- Keep list items parallel in grammatical structure

## Terminology

### Consistency Rules

- Use the same term the code uses — if it is called a "handler", the docs call it a "handler"
- Define domain-specific terms on first use
- Do not abbreviate unless the abbreviation is more common than the full term
- Use present tense ("Returns the result" not "Will return the result")
- Use active voice ("The function validates input" not "Input is validated by the function")

### Audience Adaptation

| Audience | Depth | Assumed Knowledge | Focus |
|----------|-------|-------------------|-------|
| Library consumers | API-level | Programming competence, no internals knowledge | How to use it, what it returns, what can go wrong |
| Contributors | Implementation-level | Programming competence + domain context | Why it works this way, architecture, design decisions |
| End users | Behavior-level | No programming knowledge | What it does, how to configure, how to troubleshoot |

## Quality Checklist

Before finalizing documentation, verify:

- [ ] Every public function/class/type is documented
- [ ] Every documented function has at least one example
- [ ] All parameter tables are complete (no missing types or descriptions)
- [ ] All code examples are syntactically valid
- [ ] Heading hierarchy is consistent (no skipped levels)
- [ ] No placeholder text remains (`TODO`, `TBD`, `FIXME`, `[UNCLEAR]` without explanation)
- [ ] Cross-references between related sections are present
- [ ] Error conditions are documented for every function that can fail
