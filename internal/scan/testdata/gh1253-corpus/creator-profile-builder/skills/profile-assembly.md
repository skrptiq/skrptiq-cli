---
type: skill
id: profile-assembly
title: Profile Assembly
description: "Assemble the Voice Profile and Audience Profile from analysis outputs — the core loop step that builds the deliverables"
tags: [Production, Generation, Content, Loop]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Combines the outputs from voice-pattern-analysis, audience-profile-synthesis, and banned-language-calibration into two structured deliverable documents: the Voice Profile and the Audience Profile. Uses the template assets to ensure consistent structure.

This is the generation step in the until_pass refinement loop. On the first iteration, it builds from scratch. On subsequent iterations, it revises based on validation feedback.

## When to Use

- As the assembly step after all analysis is complete
- Iterates in a loop with profile-validation until the profiles meet quality criteria

## What It Does

### First Iteration
1. Takes the structured analysis from voice-pattern-analysis
2. Takes the audience inference from audience-profile-synthesis
3. Takes the calibrated banned language list from banned-language-calibration
4. Fills in both templates (voice-profile-template and audience-profile-template) with specific, evidence-backed content
5. Ensures every section includes direct quotes from the original content samples
6. Targets 3,000-5,000 words for the Voice Profile

### Subsequent Iterations
1. Reads the validation feedback identifying weak sections, missing evidence, or insufficient depth
2. Revises only the sections flagged by the validator
3. Adds evidence, deepens analysis, or restructures as directed
4. Preserves sections that already passed validation

## Quality Standards

- Every claim must cite a direct quote from the source material
- No section may use generic filler ("the creator has a unique voice") — specifics only
- The Voice Profile's Voice Markers section must contain 5-10 ranked features with evidence
- The Banned Language section must be calibrated (not the default list copy-pasted)
- The Audience Profile's Repellents section must include concrete examples of what not to do

## What It Does Not Do

- Does not invent patterns unsupported by evidence
- Does not pad the word count with repetition or generic advice
- Does not change the creator's voice — only documents it
