---
type: skill
id: core-message-drafting
title: Core Message Drafting
description: "Drafts the core message for a social media topic using the creator's voice profile"
tags: [Production, Content, Social]
connections:
  - target: llm-service
    type: runs_on
context_params:
  voice_profile:
    label: "Voice Profile"
    description: "Creator's writing style, vocabulary, and patterns"
    required: false
  audience_profile:
    label: "Audience Profile"
    description: "Who this content is for"
    required: false
---

## Capability

Takes a topic or idea and drafts the core message — the central argument, insight, or story — in the creator's authentic voice. This is the seed that gets adapted per platform in the next step.

The core message is platform-agnostic: it captures what the creator wants to say, not how it should be formatted. Platform-specific adaptation happens downstream.

## When to Use

- As the first content generation step in the profile-aware social media pipeline
- When a creator has a topic but needs to articulate the core message before adapting it

## What It Does

1. **Extract the core insight** from the topic — what's the one thing worth saying?
2. **Draft in the creator's voice** using their sentence patterns, vocabulary, and rhetorical devices
3. **Identify the hook** — the opening that grabs attention
4. **Structure the argument** — setup, insight, evidence/example, takeaway
5. **Include supporting points** — 2-3 points that can be expanded or compressed per platform

## What It Does Not Do

- Format for any specific platform (that's platform-adaptation)
- Generate hashtags, mentions, or platform-specific CTAs
- Create images or visual content (that's image-briefing)
