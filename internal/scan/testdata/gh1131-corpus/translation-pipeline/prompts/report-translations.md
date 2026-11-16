---
type: prompt
id: report-translations
title: Report Translations
description: "Produces a comparative quality matrix across all translated languages"
tags: [Production, Reporting]
inputs:
  source_text:
    label: "Source Text"
    description: "The original text for reference"
    example: "[The original content that was translated]"
    required: true
    type: text
connections:
  - target: translation-reporting
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: synthesis
---

## Purpose

Drives the translation reporting skill. Runs after the for_each translation loop. Aggregates all quality reviews into a comparative report.

## Prompt

You are a localization manager. Produce a comparative translation quality report from the results below.

### Translation Results

{{loop.translation-cycle.results}}

### Loop Status

- **Status:** {{loop.translation-cycle.status}}
- **Languages processed:** {{loop.translation-cycle.iterations}}

### Instructions

**1. Quality Matrix**

| Language | Accuracy | Fluency | Cultural Fit | Register | Completeness | Overall | Status |
|----------|----------|---------|-------------|----------|-------------|---------|--------|
| [lang] | [score] | [score] | [score] | [score] | [score] | [avg] | Ready/Revision/Retranslation |

**2. Rankings**

Order languages by overall quality score, best to worst.

**3. Common Issues**

Patterns that appear across multiple languages (e.g. same idiom mistranslated, same section consistently weak).

**4. Publication Readiness**

- **Ready to publish:** [list of languages]
- **Needs revision:** [list with specific issues per language]
- **Needs retranslation:** [list with explanation]

**5. Effort Estimate**

For languages needing revision, estimate the work: minor (< 1 hour), moderate (1-3 hours), significant (> 3 hours).

## Formatting Rules

- Use British English throughout
- Lead with the summary — executives read the first table only
- Be specific about what needs fixing in each language
