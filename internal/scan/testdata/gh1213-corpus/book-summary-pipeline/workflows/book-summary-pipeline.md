---
type: workflow
id: book-summary-pipeline
title: Book Summary Pipeline
description: "Turn any book or PDF into a structured summary with key ideas, chapter breakdowns, and critical analysis"
tags: [Production, Academic, Learning, Reading]
connections:
  - target: content-extraction
    type: uses
  - target: synthesise-book-summary
    type: uses
  - target: language-polish
    type: uses
  - target: llm-service
    type: runs_on
  - target: effective-summarisation-guide
    type: references
  - target: citation-extraction
    type: uses
  - target: critical-thinking-framework
    type: references
  - target: summary-guide
    type: references
  - target: summary-output-template
    type: references
output_step: "language-polish"
composite_steps:
  - "content-extraction"
  - "synthesise-book-summary"
  - "citation-extraction"
  - "language-polish"
execution:
  - skill: "content-extraction"
    step_type: "synthesis"
    prompt: "extract-key-ideas"
    output: { name: "key_ideas", type: "list" }
  - skill: "synthesise-book-summary"
    prompt: "compile-summary"
    step_type: "synthesis"
    output: { name: "summary", type: "text" }
  - skill: "citation-extraction"
    prompt: "extract-citations"
    step_type: "synthesis"
    output: { name: "citations", type: "list" }
    context:
      citation_style: "Harvard"
  - skill: "language-polish"
    prompt: "polish-language"
    step_type: "content"
    output: { name: "polished_summary", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      grammar_strictness: "Professional"
---

## Overview

A three-stage pipeline that transforms a book, PDF, or ebook into a structured, revision-ready summary. Designed for students who need to extract and retain the core ideas from assigned readings without losing critical nuance.

The pipeline works with any text-based book format — paste the full text, upload a PDF, or feed in chapters individually.

## Pipeline Stages

### Stage 1: Content Extraction

The **content-extraction** skill processes the raw input and identifies the book's structure: chapters, sections, headings, and any front/back matter. It produces a structured outline that the later stages use as scaffolding.

For longer books, this stage chunks the content into manageable sections while preserving chapter boundaries.

### Stage 2: Chapter-Level Summarization

Each chapter or major section is processed individually using the **summarise-chapter** prompt. This produces:

- A one-paragraph chapter summary
- Key arguments or ideas (3–5 per chapter)
- Important terms and definitions
- Notable quotes with page/section references
- How the chapter connects to the book's central thesis

### Stage 3: Full Summary Synthesis

The **synthesise-book-summary** skill takes all chapter summaries and compiles them into a cohesive document using the **compile-summary** prompt. The final output includes:

- Book metadata (title, author, genre, publication year)
- A one-page executive summary
- Chapter-by-chapter breakdown
- Thematic analysis across the whole book
- Critical evaluation (strengths, weaknesses, gaps)
- Key takeaways for revision

## Inputs

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `input.book_text` | string | Yes | The full text of the book, or a chapter/section to process |
| `input.book_title` | string | Yes | Title of the book |
| `input.book_author` | string | Yes | Author of the book |
| `input.subject_area` | string | No | Academic subject for contextual analysis (e.g. "Psychology", "History") |
| `input.summary_depth` | string | No | Level of detail: "brief", "standard", or "detailed" (default: "standard") |
| `input.focus_areas` | string | No | Specific themes or topics to prioritize in the summary |

## Outputs

| Output | Description |
|--------|-------------|
| Executive summary | One-page overview of the book's core argument and contribution |
| Chapter summaries | Per-chapter breakdown with key ideas, terms, and quotes |
| Thematic analysis | Cross-chapter themes and how they develop through the book |
| Critical evaluation | Strengths, limitations, and gaps in the author's argument |
| Revision notes | Condensed key takeaways formatted for quick review |

## Setup

1. Import the skrpt into your workspace
2. Connect the **llm-service** node to your preferred AI provider
3. Paste or upload your book content as the input
4. Optionally set `subject_area` and `focus_areas` to tailor the analysis

## Provider Notes

This pipeline works well with any capable language model. For longer books (400+ pages), a model with a large context window will produce better results as it can hold more of the book in memory during synthesis. If your model has a smaller context window, feed chapters individually — the pipeline handles both approaches.

## Example Input

```
Book title: Thinking, Fast and Slow
Author: Daniel Kahneman
Subject area: Psychology
Summary depth: standard
Focus areas: cognitive biases, decision-making
```
