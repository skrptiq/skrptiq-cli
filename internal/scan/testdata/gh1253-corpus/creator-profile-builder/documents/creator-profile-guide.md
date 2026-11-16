---
type: document
id: creator-profile-guide
title: Creator Profile Guide
description: "How to prepare content samples and use your Voice Profile and Audience Profile"
tags: [Production, Content, Guide]
connections: []
---

# Creator Profile Guide

## What This Produces

Two separate profile documents:

1. **Voice Profile** (3,000-5,000 words) — Your writing DNA. How you construct sentences, which words you reach for, how you open and close pieces, what you never do. Deep enough that output written against this profile is recognisably yours on first read.

2. **Audience Profile** — Who you write for. Not demographics — what they already know, what they value, what drives them away. Calibrated to your actual content, not assumptions.

Both profiles are reusable assets. Feed them into other content workflows to maintain your voice across AI-assisted writing.

## Preparing Your Content Samples

The quality of your profiles depends on the quality of your input. Better samples produce sharper profiles.

### What to provide

- **3-5 pieces minimum.** More samples give the analyser more patterns to detect. 10+ pieces produce the most accurate profiles.
- **Your best and most recent work.** The profile should capture who you are now, not who you were three years ago.
- **Variety within your niche.** Different topics, different formats (if you write both tutorials and opinion pieces, include both). Same voice, different contexts — that's what reveals the constants.
- **500+ words per piece.** Short social posts don't provide enough signal for sentence-level analysis.

### What to avoid

- **Heavily edited collaborative pieces.** If an editor rewrote half of it, the voice isn't fully yours.
- **Press releases or corporate copy.** These follow house style, not your voice.
- **Very early work.** Unless your style hasn't changed, older pieces dilute the profile with patterns you've since abandoned.
- **Copy-pasted templates.** If parts of the piece follow a rigid template, the analyser might attribute the template's patterns to your voice.

### No samples? Use the interview pathway

If you don't have written samples, provide a detailed self-description instead. Describe:

- What you write about and for whom
- How you typically start and end a piece
- Words you overuse and words you hate
- Writers whose style you admire (and why)
- Whether you use humour, and what kind
- Your pet peeves in other people's writing
- How formal or casual you are, with examples

The interview pathway produces profiles with lower confidence ratings. Re-run with actual samples when you have them to sharpen the results.

## Reading Your Voice Profile

### What to check

- **Voice Summary** — Does this sound like you? Read it to someone who knows your writing and ask if they recognise you.
- **Evidence quotes** — Are these real quotes from your samples? Do they actually demonstrate the pattern claimed?
- **Anti-Patterns** — Are these things you genuinely avoid, or things you occasionally do?
- **Banned Language** — Scan the list. Is anything there that you actually use? Flag it for removal.
- **Voice Markers** — These are the top 5-10 features that make your writing yours. Do they feel right? Would missing any of them make the output feel "off"?

### Confidence ratings

- **5/5** — 10+ samples, strong consistent patterns, high analyser confidence
- **4/5** — 5-9 samples, clear patterns with some variation
- **3/5** — 3-4 samples, patterns detected but limited evidence
- **2/5** — 1-2 samples or self-description, significant inference required
- **1/5** — Brief self-description only, mostly inferred

## Reading Your Audience Profile

### What to check

- **Reader Segments** — Do these match who actually reads your work? If you have analytics, compare.
- **Repellents** — These are the hard boundaries. Is anything listed as a repellent that your audience actually tolerates?
- **Tone Calibration** — Does the formality/humour/directness mapping feel right for your readers?

## Using Your Profiles

### In other skrpts

Your profiles work as inputs to any content workflow that accepts style or audience context:

- **Blog Post Pipeline** — Provide the Voice Profile as style guidelines and the Audience Profile as target audience context.
- **Content Repurposing Flow** — The Voice Profile ensures repurposed content sounds like you across formats.
- **Newsletter Production** — Both profiles together keep your newsletter voice consistent.

### Via step.context

When a skill accepts `style_guidelines` or `voice_profile` as a context parameter, paste the relevant Voice Profile sections. You don't need to paste the entire profile — targeted injection works better:

- For **language-polish**: the Banned Language and Anti-Patterns sections
- For **editorial-review**: the Voice Markers and Structural Habits sections
- For **tone-adaptation**: the Audience Profile's Tone Calibration Map

### Updating your profiles

Re-run this workflow every 6-12 months, or when your writing style evolves significantly. Your voice changes — your profile should change with it.
