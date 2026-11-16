---
type: prompt
id: translate-text
title: Translate Text
description: "Translates text into the target language while preserving meaning, tone, and formatting"
tags: [Production, Translation]
inputs:
  source_text:
    label: "Source Text"
    description: "The text to translate"
    example: "Our platform helps teams ship faster. Build workflows visually."
    required: true
    type: file
    accept: ".txt,.md,.docx"
  source_language:
    label: "Source Language"
    description: "The language of the original text"
    example: "English"
    required: true
    type: text
context_params:
  translation_brief:
    label: "Translation Brief"
    description: "The translation brief — target language, tone, and glossary guidance from the brief step."
    required: false
connections:
  - target: text-translation
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: core
---

## Purpose

Drives the translation skill. Translates the source text into the current target language.

## Prompt

You are a professional translator. Translate the text below from {{input.source_language}} into {{loop.item}}.

### Source Text

{{input.source_text}}

### Translation Brief

{{step.context.translation_brief}}

### Instructions

1. **Translate accurately** — preserve the exact meaning of the original
2. **Maintain formatting** — keep headings, lists, bold, links, and paragraph structure
3. **Handle conventions** — adapt dates, numbers, currency, and units to the target locale's conventions
4. **Preserve tone** — match the formality and voice of the original
5. **Flag untranslatables** — if a term has no good equivalent, keep it in the original language and note it

### Rules

- Do not add, remove, or editoralise content — translate only
- Preserve all markdown formatting exactly
- If the source contains brand names or product names, keep them unchanged
- For idioms, find a culturally equivalent expression rather than translating literally

### Output

Return the COMPLETE translated text — the entire document in the target language. Do not return a partial translation, a summary, or a side-by-side comparison. The output should be the finished translated document, ready to use.

## Formatting Rules

- Use the target language throughout
- Preserve the original document structure
