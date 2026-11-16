---
type: skill
id: voice-pattern-analysis
title: Voice Pattern Analysis
description: "Deep extraction of writing DNA from content samples — sentence patterns, vocabulary fingerprint, structural habits, rhetorical devices"
tags: [Production, Analysis, Content, Voice]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Analyses a creator's written content to extract deep voice patterns — the specific, observable characteristics that make their writing recognisably theirs. This is linguistic forensics, not sentiment analysis.

The output is a structured voice analysis covering seven dimensions: sentence architecture, paragraph rhythm, vocabulary fingerprint, rhetorical devices, structural habits, opinion style, and anti-patterns. Each dimension includes concrete evidence (direct quotes from the source material).

## When to Use

- As the first analytical step when building a creator's Voice Profile
- When a creator provides new content samples and wants to update their profile
- When comparing voice consistency across different content types or time periods

## What It Does

1. **Sentence architecture** — Measures average sentence length, variation patterns, opener types (question, statement, imperative, scene-setting), closer patterns, fragment usage, compound vs simple ratios. Not counting words — identifying rhythm and construction patterns.

2. **Paragraph rhythm** — Analyses paragraph length distribution, how arguments build within paragraphs (claim-evidence-implication vs example-then-principle), transition strategies, use of one-sentence paragraphs for emphasis.

3. **Vocabulary fingerprint** — Identifies signature words and their approximate frequency, abstraction level, technical density, metaphor/analogy source domains, contraction usage, formality markers. Maps the specific words and phrases this creator reaches for.

4. **Rhetorical devices** — Catalogs question types and frequency, list formatting preferences, parenthetical asides, direct reader address, imperatives, repetition patterns, humour type and frequency.

5. **Structural habits** — Documents how they open pieces, how they close, section architecture, subheading style, typical piece length, use of visual elements (code blocks, bold, block quotes).

6. **Opinion and expertise style** — Identifies perspective (person), opinion delivery method, counterargument handling, expertise signalling patterns, teaching methodology.

7. **Anti-patterns** — Identifies structures, phrases, tones, and approaches the creator consistently avoids across all samples. Absence patterns are as diagnostic as presence patterns.

## Two Pathways

This skill handles both input types:

- **Content samples** (recommended): Performs full linguistic analysis across all provided samples, extracting patterns that appear consistently.
- **Self-description**: When only a brief description is provided instead of samples, switches to inference mode — extracting every signal from how the creator describes their style, while marking sections as lower confidence.

## What It Does Not Do

- Does not judge quality — a creator's voice is their voice, regardless of whether the patterns are "good writing"
- Does not prescribe — describes what IS, not what should be
- Does not hallucinate patterns — every claim must be supported by a direct quote from the source material
- Does not confuse topic with voice — the same voice can write about databases and cooking
