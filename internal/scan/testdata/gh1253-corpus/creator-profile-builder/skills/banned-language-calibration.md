---
type: skill
id: banned-language-calibration
title: Banned Language Calibration
description: "Personalise the default banned words list against the creator's actual vocabulary usage"
tags: [Production, Analysis, Quality, Content]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Takes the default banned words/phrases list and calibrates it against the creator's actual content. Removes words the creator genuinely uses, adds words they consistently avoid, and builds a replacement map showing what the creator says instead of each banned term.

## When to Use

- As the third analytical step in profile building (after voice analysis and audience synthesis)
- The output feeds directly into the Voice Profile's Banned Language section

## What It Does

1. **Scan content against default list** — Checks every word and phrase in the banned-words-reference asset against the creator's content samples. Records exact matches with context.

2. **Remove legitimate usage** — If a banned word appears naturally in the creator's writing (e.g., "leverage" used by a finance writer in its financial sense), it is removed from the personalised ban list with a note explaining why.

3. **Build caution list** — Words used rarely (1-2 occurrences across all samples) go on a caution list — flag for review but don't auto-reject.

4. **Detect additional avoidances** — Identify common words/phrases that never appear in the creator's content despite being common in their genre. These are potential additions to the personalised ban list.

5. **Map replacements** — For each banned word/phrase, identify what the creator actually says instead. This gives downstream skills concrete alternatives, not just prohibitions.

## What It Does Not Do

- Does not ban words based on taste or style guides — only based on the creator's actual usage patterns
- Does not override the creator's genuine vocabulary choices
- Does not ban domain-specific terminology that happens to appear on the default list
