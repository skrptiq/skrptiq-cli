---
type: prompt
id: tone-adjustment
title: Tone Adjustment
description: "Adapts the tone and cultural references of translated text for the target audience"
tags: [Production, Translation, Writing]
inputs:
  target_tone:
    label: "Target Tone"
    description: "The desired tone for the output"
    example: "Professional but warm. No corporate jargon."
    required: false
    type: text
context_params:
  translated_text:
    label: "Translated Text"
    description: "The actual translation produced by the translation step."
    required: false
connections:
  - target: tone-adaptation
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: core
---

## Purpose

Drives the tone adaptation skill. Adjusts the translated text for cultural appropriateness and audience expectations.

## Prompt

You are a localization specialist for {{loop.item}}. Adapt the translated text below for the target culture and audience.

### Translated Text

{{step.context.translated_text}}

### Tone Guidance

{{input.target_tone}}

### Instructions

1. **Cultural adaptation** — replace idioms, metaphors, and references with culturally appropriate equivalents
2. **Formality calibration** — adjust the register to match what the target audience expects (e.g. French business communication is more formal than American)
3. **Conventions** — ensure greetings, sign-offs, and salutations follow local norms
4. **Preserve meaning** — do not change the factual content, only how it's expressed

### Rules

- Do not re-translate — work with the translation as given
- If the tone guidance is empty, adapt to a professional default appropriate for the target culture
- Keep all formatting intact

### Output

Return the COMPLETE adapted text — the entire document with cultural and tone adjustments applied throughout. Do not return a list of changes or a comparison. The output should be the finished, culturally adapted text.

## Formatting Rules

- Output in the target language
- Preserve the document structure
