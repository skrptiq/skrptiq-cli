---
type: skill
id: platform-adaptation
title: Platform Adaptation
description: "Adapts the core message for a specific social media platform while maintaining the creator's voice"
tags: [Production, Content, Social, Loop]
connections:
  - target: llm-service
    type: runs_on
context_params:
  voice_profile:
    label: "Voice Profile"
    description: "Creator's writing style for voice-consistent adaptation"
    required: false
  audience_profile:
    label: "Audience Profile"
    description: "Target audience for tone calibration per platform"
    required: false
---

## Capability

Takes the core message and adapts it for a specific platform, following platform conventions while preserving the creator's voice. Runs once per platform in a for_each loop.

## When to Use

- Inside the for_each loop, once per target platform
- After the core message has been drafted

## What It Does

1. **Platform formatting** — applies platform-specific structure (character limits, thread format, paragraph style)
2. **Hook adaptation** — adapts the opening hook for the platform's scroll behavior
3. **Depth calibration** — adjusts detail level (LinkedIn: deep, Twitter: compressed, Blog: teaser)
4. **CTA matching** — adds platform-appropriate calls to action
5. **Voice preservation** — maintains the creator's vocabulary, sentence patterns, and rhetorical devices throughout

## Platform Templates

- **LinkedIn:** Professional depth, thought leadership angle, 1,300 char sweet spot, paragraph breaks, optional poll/question
- **Twitter/X:** 280 char limit per tweet, thread-friendly structure, hook in first tweet, concise supporting points
- **Blog intro:** 150-250 word teaser, drives to long-form, includes the hook and first key insight
- **Newsletter:** Personal and conversational, first-person, "here's what I've been thinking about" framing
