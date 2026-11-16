---
type: skill
id: audience-profile-synthesis
title: Audience Profile Synthesis
description: "Infer who the creator writes for based on content patterns — assumed knowledge, vocabulary level, tone calibration"
tags: [Production, Analysis, Content, Audience]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Infers the creator's audience from their content. Not who the creator says their audience is — who their content actually speaks to. Based on assumed knowledge, vocabulary level, examples chosen, problems addressed, and tone calibration.

## When to Use

- As the second analytical step when building a creator's profiles (after voice-pattern-analysis)
- When updating audience understanding based on new content samples

## What It Does

1. **Reader segmentation** — Identifies 2-3 primary reader segments based on the assumed knowledge, technical depth, and problem framing in the content. Each segment gets a role, expertise level, and specific description of what they come to this creator for.

2. **Values and motivations** — Determines what draws this audience: actionable insight, entertainment, validation, learning, career advancement, problem-solving. Based on what the content offers, not what the creator claims.

3. **Pain point mapping** — Identifies the core problems the content addresses, urgency level, and current alternatives the audience likely uses.

4. **Repellent identification** — Infers what would drive this audience away based on what the creator carefully avoids: condescension, fluff, hype, oversimplification, jargon without context.

5. **Tone calibration** — Maps where the content sits on formality, humour, directness, jargon usage, and evidence expectations spectrums. Produces a calibration map that other skills can consume.

6. **Content preference inference** — From the creator's output patterns, infers preferred formats, length, depth, and frequency expectations.

## What It Does Not Do

- Does not replace actual audience research or analytics
- Does not assume demographics from content topics
- Does not project the creator's preferences onto the audience
