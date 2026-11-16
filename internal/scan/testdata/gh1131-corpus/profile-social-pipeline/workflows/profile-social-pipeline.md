---
type: workflow
id: profile-social-pipeline
title: Profile-Aware Social Media Pipeline
description: "Generate personalized multi-platform social content using your voice and audience profiles"
tags: [Production, Customer-Facing, Content, Social, Loop]
connections:
  - target: core-message-drafting
    type: uses
  - target: platform-adaptation
    type: uses
  - target: voice-verification
    type: uses
  - target: language-polish
    type: uses
  - target: consistency-check
    type: uses
  - target: llm-service
    type: runs_on
metadata:
  estimated_duration: "5-10 minutes"
  trigger: manual
  loop_modes: ["for_each"]
loops:
  - id: "platform-loop"
    mode: "for_each"
    inputExpression: "{{input.platforms}}"
    steps:
      - "platform-adaptation"
    maxIterations: 10
output_step: "language-polish"
composite_steps:
  - "core-message-drafting"
  - "platform-adaptation"
  - "voice-verification"
  - "language-polish"
  - "consistency-check"
execution:
  - skill: "core-message-drafting"
    prompt: "draft-core-message"
    step_type: "generation"
    output: { name: "core_message", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      audience_profile: "General professional audience"
  - skill: "platform-adaptation"
    prompt: "adapt-for-platform"
    step_type: "generation"
    output: { name: "platform_content", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      audience_profile: "General professional audience"
  - skill: "voice-verification"
    prompt: "verify-voice"
    step_type: "validation"
    output: { name: "voice_verdict", type: "decision" }
    context:
      voice_profile: "Neutral professional tone"
  - skill: "language-polish"
    prompt: "polish-language"
    step_type: "content"
    output: { name: "polished_posts", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      grammar_strictness: "Professional"
    bindings:
      source:
        from_step: "Platform Adaptation"
        field: output
  - parallel:
    - skill: "consistency-check"
      prompt: "check-consistency"
      step_type: "review"
      output: { name: "consistency_verdict", type: "decision" }
      context:
        voice_profile: "Neutral professional tone"
        consistency_strictness: "Standard"
---

## Overview

This workflow generates personalized social media content across multiple platforms from a single topic — all in your authentic voice. It uses your **Voice Profile** and **Audience Profile** to produce posts that sound like you wrote them, not like a generic social media tool.

One topic in, platform-specific posts out. Same voice, different formats.

## How Profiles Apply

- **Voice Profile** feeds into every step — core drafting, platform adaptation, and voice verification all use your writing DNA
- **Audience Profile** calibrates tone and depth per platform — your LinkedIn audience may differ from your newsletter subscribers
- If no profiles are set, the pipeline still works using standard editorial judgement

## Pipeline Stages

### Stage 1: Core Message Drafting

**Input:** Topic, target platforms, key points, CTA

Invoke the **core-message-drafting** skill via the **draft-core-message** prompt. Drafts the central message in the creator's voice — the hook, setup, insight, evidence, and takeaway. Platform-agnostic.

**Output:** Core message (300-500 words) with platform adaptation notes.

### Stage 2: Platform Adaptation (For Each)

**Input:** Core message, iterated over each target platform

The platform list (`{{input.platforms}}`) is provided as a JSON array — or one platform per line — so the loop can iterate it. The current platform is available to each iteration via `{{loop.item}}`.

The **platform-adaptation** skill runs once per platform via the **adapt-for-platform** prompt. Each iteration produces platform-specific content:

- **LinkedIn:** 1,200-1,500 characters, professional depth, thought leadership
- **Twitter/X:** Single tweet or 3-7 tweet thread, hook-driven
- **Blog intro:** 150-250 word teaser driving to long-form
- **Newsletter:** 200-400 words, personal and conversational

**Output:** Adapted post for each platform, formatted and ready to publish.

### Stage 3: Voice Verification

**Input:** All platform adaptations

Invoke the **voice-verification** skill via the **verify-voice** prompt. Reviews every adaptation against the Voice Profile — vocabulary, sentence patterns, rhetorical devices, banned words, and anti-patterns.

**Output:** Per-platform voice match scores, specific issues, and a pass/needs-revision verdict.

### Stage 4: Language Polish

Invoke **language-polish** to clean up the final content across all platforms.

### Stage 5: Consistency Check (Parallel)

Invoke **consistency-check** to verify the core message is consistent across all platform adaptations.

## Error Handling

- If no platforms are specified, defaults to LinkedIn + Twitter/X
- If Voice Profile is not set, content is drafted in a clear, direct style
- If voice verification flags issues, the report includes specific rewording suggestions
- If a platform is not recognized, it's adapted using general social media best practices

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.topic}}` | Yes | The topic or insight to share | `Most teams waste time in meetings because they confuse discussion with decision-making` |
| `{{input.platforms}}` | Yes | Target platforms, as a JSON array or one per line | `["LinkedIn", "Twitter", "Blog intro", "Newsletter"]` |
| `{{input.key_points}}` | No | Specific points to include | `1. Discussion vs decision meetings. 2. Amazon's 6-pager. 3. Cutting 60% of meetings.` |
| `{{input.cta}}` | No | What readers should do | `Try the decision meeting format for one week` |

## Outputs

| Name | Description |
|------|-------------|
| Platform-specific posts | Adapted content for each target platform, formatted and ready to publish |
| Voice verification report | Per-platform voice match scores and any issues flagged |

## Setup

1. For best results, create a Voice Profile and Audience Profile first using the **Creator Profile Builder**.
2. Profiles apply automatically when set — no manual configuration needed per run.
3. No external services required.

## Provider Notes

- Core message drafting and platform adaptation benefit from stronger models for voice matching.
- Voice verification runs a detailed comparison — capable models produce more specific feedback.
- The for_each loop runs once per platform, so total cost scales linearly with platform count.

## Example Input

```
Topic: "Most teams waste time in meetings because they confuse discussion with decision-making. A discussion meeting explores options. A decision meeting commits to one. Mixing them guarantees neither happens well."
Platforms: ["LinkedIn", "Twitter", "Blog intro", "Newsletter"]
Key Points: "1. The distinction between 'meeting to discuss' and 'meeting to decide' — most teams only have the first kind. 2. Amazon's approach: write the decision memo before the meeting, use meeting time to decide, not to present. 3. My experience: when I started labeling meetings as 'discussion' or 'decision', we cut 60% of them — most discussions didn't need a meeting at all."
Call To Action: "Next week, label every meeting on your calendar as either 'discussion' or 'decision'. Cancel any discussion meeting that could be a document instead. Tell me what happens."
```
