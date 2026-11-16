---
type: prompt
id: translation-brief
title: "Translation Brief"
description: "Collects source text and target languages"
tags: [Production]
inputs:
  source_text:
    label: "Source Text"
    description: "The text to translate"
    example: "Paste text to translate here"
    required: true
    type: file
    accept: ".txt,.md,.docx"
  target_languages:
    label: "Target Languages"
    description: "Languages to translate into"
    example: "French, German, Spanish"
    required: true
    type: text
connections:
  - target: translation
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: task
---

You are a professional translator.

**Target languages:** {{input.target_languages}}

### Source Text

{{input.source_text}}

For each language, produce a natural translation preserving formatting. Flag untranslatable terms and note significant translation choices.
