---
type: prompt
id: review-profiles
title: "Review Profiles"
description: "Gate prompt — shown to the creator when execution pauses for profile review"
tags: [Production, Gate, Review]
connections:
  - target: profile-review
    type: derived_from
metadata:
  output_format: text
  prompt_type: gate
---

## Your Creator Profiles Are Ready for Review

Please review the Voice Profile and Audience Profile below. These are your deliverables — they should accurately capture your writing DNA and your audience.

### What to check

**Voice Profile:**
- **Voice Summary** — Does this sound like you? Read it aloud. Would someone who knows your writing recognise you from this description?
- **Evidence quotes** — Are these real quotes from your samples? Do they demonstrate the patterns claimed?
- **Sentence Architecture & Vocabulary** — Do the specific measurements and patterns feel accurate?
- **Anti-Patterns** — Are these things you genuinely never do, or things you occasionally do?
- **Banned Language** — Scan the list. Is anything banned that you actually use? Is anything missing that you hate?
- **Voice Markers** — These are the top features that make your writing yours. Are they the right ones? In the right order?

**Audience Profile:**
- **Reader Segments** — Do these match who actually reads your work?
- **Repellents** — Are these genuine dealbreakers for your audience?
- **Tone Calibration** — Does the formality/humour/directness mapping feel right?

### How to respond

- **If the profiles are accurate:** respond with "approved" or "looks good"
- **If changes are needed:** describe what's wrong and what should change. Be specific — "the Voice Summary makes me sound more formal than I am" is better than "it's not quite right"

The more specific your feedback, the better the revision.

---

{{steps.previous.output}}
