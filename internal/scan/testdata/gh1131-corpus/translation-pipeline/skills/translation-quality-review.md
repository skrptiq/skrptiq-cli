---
type: skill
id: translation-quality-review
title: Translation Quality Review
description: "Reviews a translation for accuracy, cultural fit, and natural expression via back-translation and adaptation check"
tags: [Production, Quality]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Evaluates a completed translation by performing a back-translation to the source language and comparing it against the original. Checks for meaning drift, cultural appropriateness, register consistency, and natural expression. Produces a quality score and specific improvement recommendations.

## When to Use

- After translating content into a new language
- As a quality gate before publishing translated content
- When translation quality needs to be quantified and compared across languages

## Inputs

- The original source text
- The translation to review
- The target language and cultural context

## Outputs

A structured quality assessment:
- Back-translation comparison
- Quality score (1-10) across five dimensions: accuracy, fluency, cultural fit, register, completeness
- Specific issues found (with location and suggestion)
- Overall assessment: ready / needs revision / needs retranslation
