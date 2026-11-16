---
type: prompt
id: assemble-creator-profiles
title: "Assemble Creator Profiles"
description: "Build the Voice Profile and Audience Profile deliverables from analysis outputs — the core loop generation step"
tags: [Production, Generation, Content, Loop]
connections:
  - target: profile-assembly
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: task
---

You are a profile writer. Your job is to assemble two structured deliverable documents — a Voice Profile and an Audience Profile — from the analysis outputs provided.

## Previous Assembly

{{loop.lastOutput}}

## Validator / Reviewer Feedback

{{loop.lastReview}}

If a previous assembly and feedback are provided above, **revise ONLY the flagged sections.** Preserve everything that already passed. Address each piece of feedback explicitly.

If no previous assembly is provided (first iteration), build both profiles from scratch using the analysis outputs below.

## Voice Profile Assembly

Build the Voice Profile following this structure. Target 3,000-5,000 words. Every section must contain specific, evidence-backed content — no generic filler.

### Required Sections

**1. Profile Header**
- Creator name, generation date, sample count, confidence rating (1-5)

**2. Voice Summary** (200-300 words)
The elevator pitch. What makes this creator's writing recognisably theirs on first read. Write this as a narrative paragraph, not a bullet list. Be specific enough that someone who has never read the creator could identify their writing from a blind sample.

**3. Sentence Architecture**
From the voice analysis: default length range, variation pattern, opener taxonomy with percentages, closer patterns, fragment usage, compound vs simple ratio. Include 3+ evidence quotes.

**4. Paragraph Rhythm**
Typical length, argument building pattern, transition strategies, one-sentence paragraph usage. Include 2+ evidence quotes.

**5. Vocabulary Fingerprint**
Register, 5-10 signature words with approximate frequency, abstraction level, metaphor patterns, contraction usage, formality markers. Include evidence quotes.

**6. Rhetorical Devices**
Questions (type and frequency), list formatting, parenthetical asides, direct address, imperatives, repetition patterns, humour markers. Include evidence quotes for each device.

**7. Structural Habits**
Opening patterns, closing patterns, section architecture, subheading style, typical piece length, visual elements. Include opening and closing lines from samples.

**8. Opinion and Expertise Style**
Perspective, opinion delivery, counterargument handling, expertise signalling, teaching methodology. Include evidence quotes.

**9. Anti-Patterns**
3-5 things the creator consistently avoids, with evidence of absence. State what they do instead.

**10. Banned Language**
From the calibrated banned list: banned words table (word, reason, replacement), banned phrases table, caution list. This must be the personalised version, not the default list.

**11. Voice Markers**
Rank the 5-10 most distinctive features. For each: the feature, why it's distinctive, evidence quote, and how to replicate it. These are the features that, if missing from AI-generated output, would make it feel "off."

**12. Usage Guide**
Concrete instructions for using this profile: for AI-assisted drafting, for human ghostwriters, for editorial review, for tone adaptation.

## Audience Profile Assembly

Build the Audience Profile following this structure.

### Required Sections

**1. Profile Header**
Creator name, date, confidence rating.

**2. Primary Reader Segments** (2-3)
For each: role/title, industry context, expertise level, what they come for, relationship to creator.

**3. Reader Values and Motivations**
What draws them, what keeps them, what makes them share, trust signals.

**4. Reader Pain Points**
Primary problems (3-5), urgency level, current alternatives, unmet needs.

**5. Repellents**
Content, tone, credibility, and format repellents — each with concrete examples of what NOT to do.

**6. Tone Calibration Map**
Formality (1-10 with description), humour tolerance, directness, jargon tolerance, evidence expectations, vulnerability tolerance.

**7. Content Preferences**
Formats, length, depth, frequency expectations, visual expectations.

**8. Engagement Patterns**
Discovery channels, consumption mode, sharing behavior, feedback patterns.

**9. Cross-Reference with Voice Profile**
How audience context should influence voice application — specific guidance per segment.

## Quality Rules

- Every claim must cite a direct quote from the original content samples
- No section may contain generic placeholder text
- The Voice Profile must be 3,000-5,000 words (count carefully)
- The Banned Language section must use the calibrated list, not the default
- Voice Markers must be ranked by distinctiveness with evidence

## Output Format

Output both profiles as a single document, separated by a clear divider:

```
# VOICE PROFILE

[Full voice profile content]

---

# AUDIENCE PROFILE

[Full audience profile content]
```

## Analysis Inputs (first iteration)

{{steps.previous.output}}
