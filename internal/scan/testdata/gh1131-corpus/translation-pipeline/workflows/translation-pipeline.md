---
type: workflow
id: translation-pipeline
title: Translation Pipeline
description: "Translate content into multiple languages with quality review and comparative reporting"
tags: [Production, Customer-Facing, Translation, Loop]
connections:
  - target: translation
    type: uses
  - target: text-translation
    type: uses
  - target: tone-adaptation
    type: uses
  - target: translation-quality-review
    type: uses
  - target: translation-reporting
    type: uses
  - target: language-polish
    type: uses
  - target: llm-service
    type: runs_on
  - target: company-style-guide
    type: references
  - target: brand-guidelines
    type: references
metadata:
  estimated_duration: "5-30 minutes"
  trigger: manual
  loop_modes: ["for_each"]
loops:
  - id: "translation-cycle"
    mode: "for_each"
    inputExpression: "{{input.target_languages}}"
    steps:
      - "translation"
      - "text-translation"
      - "tone-adaptation"
      - "language-polish"
      - "translation-quality-review"
    maxIterations: 20
output_step: "translation-reporting"
composite_steps:
  - "translation"
  - "text-translation"
  - "tone-adaptation"
  - "translation-quality-review"
  - "translation-reporting"
  - "language-polish"
execution:
  - skill: "translation"
    step_type: "content"
    prompt: "translation-brief"
    output: { name: "translation_brief", type: "text" }
  - skill: "text-translation"
    prompt: "translate-text"
    step_type: "content"
    output: { name: "translation", type: "text" }
    bindings:
      translation_brief:
        from_step: "Translation"
        field: output
  - skill: "tone-adaptation"
    prompt: "tone-adjustment"
    step_type: "content"
    output: { name: "toned_translation", type: "text" }
    context:
      target_tone: "Professional and approachable"
    bindings:
      translated_text:
        from_step: "Text Translation"
        field: output
  - skill: "language-polish"
    prompt: "polish-language"
    step_type: "content"
    output: { name: "polished_translation", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      grammar_strictness: "Professional"
    bindings:
      source:
        from_step: "Tone Adaptation"
        field: output
  - skill: "translation-quality-review"
    prompt: "review-translation"
    step_type: "review"
    output: { name: "quality_review", type: "text" }
    bindings:
      translated_text:
        from_step: "Language Polish"
        field: output
  - skill: "translation-reporting"
    prompt: "report-translations"
    step_type: "synthesis"
    output: { name: "translation_report", type: "text" }
---

## Overview

This workflow translates content into multiple languages using a **for_each** loop. For each target language, it translates the source text, adapts the tone for the target culture, polishes the language, and runs a quality review with back-translation. After all languages are processed, a post-loop reporting step produces a comparative quality matrix.

## Pipeline Stages

### Stage 1: Translation Loop (for_each over target languages)

The loop iterates over the list of target languages. For each language:

**Step 1 — Translation Brief**

Prepares the translation brief for `{{loop.item}}` (the current target language): target language, tone guidance, and notes on glossary or untranslatable terms. This brief guides the actual translation.

**Step 2 — Translate**

Produces the actual translation of the source text into `{{loop.item}}`, guided by the brief, preserving formatting, structure, and meaning. Handles locale-specific conventions for dates, numbers, and units.

**Step 3 — Adapt Tone**

Adjusts the translated content for the target culture's communication norms. Modifies formality level, idioms, and references to be culturally appropriate.

**Step 4 — Polish Language**

Surface-level polish of the tone-adapted translation — fixes grammar, spelling, and punctuation in the target language.

**Step 5 — Quality Review**

Back-translates the output to the source language and compares against the original. Scores accuracy, fluency, cultural fit, register, and completeness. Produces a structured quality assessment with specific issues.

Each iteration is independent — the translation of French does not depend on or see the translation of German.

### Stage 2: Comparative Report (post-loop)

After all languages are processed, the reporting step receives all quality reviews via `{{loop.translation-cycle.results}}`. It produces:

- Quality matrix comparing all languages across five dimensions
- Rankings by overall quality score
- Common issues across multiple translations
- Publication readiness assessment per language
- Effort estimates for languages needing revision

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.source_text}}` | Yes | The text to translate | A product page, email campaign, or help article |
| `{{input.source_language}}` | Yes | The language of the source text | English |
| `{{input.target_languages}}` | Yes | List of languages to translate into (one per line or JSON array) | `["French", "German", "Japanese", "Portuguese"]` |
| `{{input.target_tone}}` | No | Tone and style guidance for translations | "Professional but approachable. Match the brand voice." |

## Outputs

| Name | Description |
|------|-------------|
| Individual translations | One translated, tone-adapted, polished text per language |
| Quality assessments | Back-translation comparison and quality score per language |
| Comparative report | Quality matrix, rankings, and publication readiness across all languages |

## Setup

1. **Prepare your source text** — have the content ready to paste
2. **List your target languages** — provide as a JSON array or one per line
3. **Review brand guidelines** — if you have tone or voice guidelines, include them

## Provider Notes

- Each language iteration runs 5 AI calls (brief + translate + adapt + polish + review). 5 languages = 25 calls plus the report.
- Multilingual capability varies by model — test with your specific language pairs
- For best results, use a model with strong multilingual training
- Quality review uses back-translation, which doubles the translation work but catches meaning drift

## Example Input

```
source_text: "Our platform helps teams ship faster. Build workflows visually, automate repetitive tasks, and focus on what matters. Join 5,000 teams who have already made the switch."
source_language: "English"
target_languages: ["French", "German", "Japanese", "Spanish"]
target_tone: "Professional but warm. No corporate jargon. Address the reader directly."
```
