---
type: skill
id: voice-verification
title: Voice Verification
description: "Checks all platform adaptations against the creator's voice profile for authenticity"
tags: [Production, Quality, Validation]
connections:
  - target: llm-service
    type: runs_on
context_params:
  voice_profile:
    label: "Voice Profile"
    description: "Creator's voice profile to verify against"
    required: false
---

## Capability

Reviews all platform-adapted posts against the creator's voice profile. Checks that each adaptation sounds like the creator wrote it — not like a generic social media manager.

## When to Use

- After all platform adaptations are complete
- As a quality gate before final polish

## What It Checks

1. **Vocabulary match** — does the adaptation use the creator's preferred words and avoid their banned list?
2. **Sentence patterns** — do sentence lengths and structures match the creator's typical patterns?
3. **Rhetorical consistency** — are the creator's characteristic devices present (questions, direct address, humour)?
4. **Anti-pattern compliance** — does the adaptation avoid things the creator never does?
5. **Platform-voice balance** — has platform adaptation gone too far, overriding the creator's voice?

## Output

For each platform adaptation:
- **Voice match score** (1-5)
- **Specific issues** — what doesn't sound like the creator
- **Suggested fixes** — concrete rewording suggestions
- **Banned word violations** — any words from the banned list that slipped through
