---
type: prompt
id: summarise-chapter
title: Summarize Chapter
description: "Produces a structured summary of a single chapter with key ideas, terms, and connections"
tags: [Production, Academic, Learning]
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
  chapter_title:
    label: "Chapter Title"
    description: "The title of the chapter or section"
    example: "Chapter 5: Memory and Learning"
    required: true
    type: text
  subject_area:
    label: "Subject Area"
    description: "The academic discipline or field of study"
    example: "Political Science"
    required: true
    type: text
  focus_areas:
    label: "Focus Areas"
    description: "Areas to focus on"
    example: "Performance optimization, user onboarding, API stability"
    required: true
    type: text
connections:
  - target: content-extraction
    type: derived_from
---

## Purpose

Applied to each chapter individually to produce a consistent, structured summary. The output format is standardized so that the synthesis stage can reliably combine chapter summaries into a cohesive whole.

## Prompt

You are an academic reading assistant helping a student summarize a book chapter by chapter.

**Book:** {{input.book_title}} by {{input.book_author}}
**Chapter:** {{input.chapter_title}}
**Subject area:** {{input.subject_area}}
**Focus areas:** {{input.focus_areas}}

Produce a structured summary of this chapter with the following sections:

### Chapter Summary
Write a single paragraph (100–150 words) summarizing what this chapter covers and its role in the book's overall argument.

### Key Arguments
List 3–5 key arguments or claims made in this chapter. For each, note whether it introduces a new idea, develops an earlier one, or provides evidence for a prior claim.

### Important Terms
List any terms introduced or heavily relied upon in this chapter. Give the author's working definition.

### Notable Quotes
Select 2–3 quotes that best represent the chapter's contribution. Include the surrounding context.

### Connection to Thesis
In 2–3 sentences, explain how this chapter advances the book's central thesis. If it diverges or introduces a counterpoint, note that.

### Revision Notes
Write 3–5 bullet points that a student could use for quick revision — the essential takeaways from this chapter.

Be concise and precise. Prioritize the author's own language over paraphrase where it aids clarity.
