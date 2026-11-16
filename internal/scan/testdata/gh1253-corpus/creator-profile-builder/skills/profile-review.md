---
type: skill
id: profile-review
title: Profile Review
description: "Human gate — pauses execution for the creator to review their profiles and approve or request changes"
tags: [Production, Gate, Review, Loop]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Pauses the workflow and presents the assembled Voice Profile and Audience Profile to the creator for review. The creator either approves the profiles or provides specific feedback for revision.

This is a **gate step** — execution pauses until the creator responds. The creator's response becomes this step's output, which the profile-assembly step uses on the next loop iteration.

## When to Use

- As the verifier in the profile refinement loop
- After profile-assembly produces or revises the profiles

## What Happens

1. Execution pauses with status `awaiting_input`
2. The creator sees their assembled profiles in the Runs view
3. The creator reviews the profiles and responds:
   - **"approved"** or similar affirmation → loop exits, profiles proceed to language-polish
   - **Specific feedback** → loop continues, profile-assembly incorporates the feedback
4. The creator's response becomes this step's output

## Review Guidance

The prompt guides the creator on what to check:

- Does the Voice Summary sound like you?
- Are the evidence quotes accurate?
- Is anything in the Banned Language list that you actually use?
- Does the Audience Profile match who you really write for?
- Are the Voice Markers the right top features?
