---
type: document
id: documentation-checklist
title: Documentation Checklist
description: "Working checklist to track documentation completeness before publication"
tags: [Production, Documentation, Quality]
connections: []
---

## Purpose

A checklist to verify documentation quality before publishing. Use this as a final gate after running the pipeline.

## Structural Completeness

- [ ] Document has a clear title (H1)
- [ ] Overview section explains what the code does and when to use it
- [ ] API reference covers every public function, class, and type
- [ ] Every function has: description, parameters, return type, and at least one example
- [ ] Error conditions are documented for functions that can fail
- [ ] Edge cases and gotchas section is present and non-empty

## Examples Quality

- [ ] Quick start example is present and copy-paste runnable
- [ ] Common pattern examples cover the 2-3 most frequent use cases
- [ ] All examples include necessary imports and setup
- [ ] Examples use realistic data (not foo/bar/test placeholders)
- [ ] Expected output is shown where it aids understanding
- [ ] Error handling is demonstrated for I/O and network operations

## Formatting

- [ ] Heading hierarchy is consistent (H1 → H2 → H3, no skips)
- [ ] All code blocks have language identifiers
- [ ] Parameter tables are complete and consistently formatted
- [ ] Inline code formatting used for function names, types, and paths
- [ ] No trailing whitespace or empty sections
- [ ] Table of contents is present with working links

## Accuracy

- [ ] Function signatures match the actual code
- [ ] Parameter types match the actual code
- [ ] Default values are correctly documented
- [ ] Return types are accurately described
- [ ] No speculative behavior — anything uncertain is flagged as [UNCLEAR]
- [ ] Terminology matches what the code uses
