---
type: prompt
id: review-translation
title: Review Translation
description: "Reviews a translation via back-translation and cultural adaptation check"
tags: [Production, Quality]
inputs:
  source_text:
    label: "Source Text"
    description: "The original text that was translated"
    example: "[Paste the original source text here]"
    required: true
    type: file
    accept: ".txt,.md,.docx"
context_params:
  translated_text:
    label: "Translated Text"
    description: "The finished translation to review — the tone-adapted, polished target-language text."
    required: false
connections:
  - target: translation-quality-review
    type: derived_from
metadata:
  output_format: json
  prompt_type: validation
---

## Purpose

Drives the translation quality review skill. Reviews each translation by back-translating and comparing against the original.

## Prompt

You are a professional translation reviewer. Evaluate the translation below for accuracy, fluency, and cultural appropriateness.

### Original Text

{{input.source_text}}

### Translation ({{loop.item}})

{{step.context.translated_text}}

### Instructions

1. **Back-translate** — translate the output back into the source language without looking at the original
2. **Compare** — identify any meaning drift between the original and the back-translation
3. **Check cultural fit** — are idioms, references, and conventions appropriate for the target culture?
4. **Check register** — does the formality level match the original?
5. **Check completeness** — is anything missing or added?

### Output Format

Produce a JSON quality assessment:

```json
{
  "language": "{{loop.item}}",
  "scores": {
    "accuracy": 8,
    "fluency": 7,
    "cultural_fit": 9,
    "register": 8,
    "completeness": 10
  },
  "overall_score": 8.4,
  "status": "ready | needs_revision | needs_retranslation",
  "issues": [
    {
      "location": "paragraph 2",
      "type": "meaning_drift",
      "original": "...",
      "translated": "...",
      "suggestion": "..."
    }
  ],
  "back_translation_excerpt": "First paragraph back-translated..."
}
```

### Rules

- Score each dimension 1-10 (10 = perfect)
- Be specific about issues — quote the problematic text
- "Ready" means publishable with no changes. "Needs revision" means minor fixes. "Needs retranslation" means fundamental problems.

## Formatting Rules

- Output must be valid JSON
- Use British English for any narrative text
