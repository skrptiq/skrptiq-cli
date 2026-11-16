---
type: prompt
id: compile-summary
title: Compile Summary
description: "Assembles chapter summaries into a complete book summary with thematic analysis and critical evaluation"
tags: [Production, Academic, Writing]
inputs:
  book_title:
    label: "Book Title"
    description: "The title of the book"
    example: "Thinking, Fast and Slow"
    required: true
    type: text
  book_author:
    label: "Book Author"
    description: "The author of the book"
    example: "Daniel Kahneman"
    required: true
    type: text
  subject_area:
    label: "Subject Area"
    description: "The academic discipline or field of study"
    example: "Political Science"
    required: true
    type: text
  summary_depth:
    label: "Summary Depth"
    description: "How detailed the summary should be"
    example: "Chapter-by-chapter with key arguments and evidence"
    required: true
    type: text
  focus_areas:
    label: "Focus Areas"
    description: "Areas to focus on"
    example: "Performance optimization, user onboarding, API stability"
    required: true
    type: text
connections:
  - target: synthesise-book-summary
    type: derived_from
---

## Purpose

The final prompt in the pipeline. Takes all chapter summaries and the extracted key ideas, then produces the finished book summary document. The depth of output is controlled by the `summary_depth` parameter.

## Prompt

You are an academic reading assistant. Compile the following chapter summaries into a complete, structured book summary.

**Book:** {{input.book_title}} by {{input.book_author}}
**Subject area:** {{input.subject_area}}
**Summary depth:** {{input.summary_depth}}
**Focus areas:** {{input.focus_areas}}

### Chapter Summaries

{{steps.previous.output}}

Using the chapter summaries provided above, produce a book summary with these sections:

### Book Overview
- Title, author, publication year (if known), genre/field
- One paragraph describing the book's purpose and intended audience

### Executive Summary
A single paragraph (150–200 words) capturing the book's central thesis, method, key findings, and significance. This should be self-contained — a reader should understand the book's contribution from this alone.

### Chapter-by-Chapter Breakdown
For each chapter, provide:
- A concise summary (2–3 sentences)
- How it connects to the chapters before and after it

### Thematic Analysis
Identify 3–5 themes that run across multiple chapters. For each theme:
- Name and define it
- Cite which chapters address it and how
- Note how the author's treatment of this theme evolves through the book

### Critical Evaluation
- **Strengths:** What does the book do well? Where is the evidence strongest?
- **Weaknesses:** Where are the arguments thin, the evidence weak, or the logic questionable?
- **Gaps:** What does the book not address that a critical reader should notice?
- **Context:** How does this book sit within its field? Does it challenge, extend, or summarize existing thinking?

### Key Takeaways
Write 5–10 bullet points summarizing the most important things a student should remember from this book. These should be useful for exam revision or essay writing.

### Glossary
List the most important specialist terms from across the book with their definitions.

Adjust the length and detail based on the summary depth:
- **Brief:** Executive summary + key takeaways only
- **Standard:** All sections, concise treatment
- **Detailed:** All sections with extended analysis, additional quotes, and methodology discussion
