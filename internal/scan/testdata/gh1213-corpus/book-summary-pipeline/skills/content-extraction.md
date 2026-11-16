---
type: skill
id: content-extraction
title: Content Extraction
description: "Parses raw book text to identify structure, chapters, and sections for downstream processing"
tags: [Production, Academic, Reading]
connections:
  - target: llm-service
    type: runs_on
  - target: effective-summarisation-guide
    type: references
metadata:
  estimated_duration: "3-8 minutes"
  avg_tokens: 5000
---

## Capability

Analyzes raw book text — whether pasted in full, uploaded as a PDF, or provided chapter by chapter — and produces a structured representation of the book's organization. This gives downstream skills a reliable map of the content.

## Processing Approach

### Structure Detection

1. Identify chapter boundaries from headings, numbering, or formatting cues
2. Detect sub-sections within each chapter
3. Recognize front matter (preface, introduction, foreword) and back matter (appendices, bibliography, index)
4. Flag any epigraphs, pull quotes, or sidebars

### Content Chunking

For books that exceed the model's context window:

1. Split at chapter boundaries (never mid-paragraph)
2. Preserve section headings as metadata on each chunk
3. Tag each chunk with its position in the book (e.g. "Chapter 3 of 12, Section 2 of 4")
4. Maintain a running outline so later stages know what came before

### Output Structure

Produces a JSON-like outline:

- Book title, author, and detected genre
- Ordered list of chapters with titles and estimated word counts
- Sub-sections within each chapter
- Content type flags (narrative, argumentative, mixed, reference)

## When to Use

- Before any summarization step, to establish the book's skeleton
- When processing a long PDF that needs to be chunked for a smaller context window
- When you have the full text and need to identify which parts are worth summarizing in depth

## Constraints

- Expects text input, not scanned images — OCR should be done beforehand
- Cannot detect visual elements (charts, diagrams, tables) from plain text
- Works best with non-fiction and structured academic texts; novels with unconventional formatting may need manual chapter markers
