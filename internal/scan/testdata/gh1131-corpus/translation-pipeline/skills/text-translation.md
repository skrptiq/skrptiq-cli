---
type: skill
id: text-translation
title: Text Translation
description: "Produces the actual target-language translation of the source text, preserving meaning, tone, and formatting"
tags: [Production, Translation, Quality]
connections:
  - target: llm-service
    type: runs_on
metadata:
  estimated_duration: "3 minutes"
  avg_tokens: 3000
  trigger: manual
---

## Text Translation

This skill performs the actual translation: it converts the source text into the current target language while preserving meaning, tone, structure, and formatting.

### Core Capability

Given the source text plus the translation brief (target language, tone, and glossary guidance), this skill produces a complete, faithful translation ready for downstream tone adaptation and quality review.

### Method

1. **Translate faithfully:** Preserve the exact meaning of the original — no additions, omissions, or editorialising.
2. **Preserve formatting:** Keep headings, lists, emphasis, links, and paragraph structure intact.
3. **Apply the brief:** Honour the target language, tone, and any glossary or untranslatable-term guidance from the brief.
4. **Handle conventions:** Adapt dates, numbers, currency, and units to the target locale.

### Output Structure

The complete translated document in the target language, ready to feed the tone-adaptation and quality-review stages downstream.
