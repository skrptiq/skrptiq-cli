---
type: workflow
id: creator-profile-builder
title: Creator Profile Builder
description: "Analyze your content to build a reusable Voice Profile and Audience Profile — deep writing DNA, not surface labels"
tags: [Production, Customer-Facing, Content, Analysis, Loop]
connections:
  - target: voice-pattern-analysis
    type: uses
  - target: audience-profile-synthesis
    type: uses
  - target: banned-language-calibration
    type: uses
  - target: profile-assembly
    type: uses
  - target: profile-validation
    type: uses
  - target: profile-review
    type: uses
  - target: language-polish
    type: uses
  - target: consistency-check
    type: uses
  - target: llm-service
    type: runs_on
  - target: voice-profile-template
    type: references
  - target: audience-profile-template
    type: references
  - target: banned-words-reference
    type: references
  - target: creator-profile-guide
    type: references
metadata:
  estimated_duration: "5-15 minutes"
  trigger: manual
  loop_modes: ["until_pass"]
loops:
  - id: "profile-refinement"
    mode: "until_pass"
    steps:
      - "profile-assembly"
      - "profile-validation"
      - "profile-review"
    verifier: "profile-review"
    maxIterations: 4
    freshContextPerIteration: true
output_step: "language-polish"
composite_steps:
  - "voice-pattern-analysis"
  - "audience-profile-synthesis"
  - "banned-language-calibration"
  - "profile-assembly"
  - "profile-validation"
  - "profile-review"
  - "language-polish"
  - "consistency-check"
execution:
  - skill: "voice-pattern-analysis"
    prompt: "analyse-voice-patterns"
    step_type: "synthesis"
    output: { name: "voice_patterns", type: "text" }
  - skill: "audience-profile-synthesis"
    prompt: "extract-audience-signals"
    step_type: "synthesis"
    output: { name: "audience_profile", type: "text" }
  - skill: "banned-language-calibration"
    prompt: "calibrate-banned-words"
    step_type: "synthesis"
    output: { name: "banned_words", type: "list" }
  - skill: "profile-assembly"
    prompt: "assemble-creator-profiles"
    step_type: "generation"
    output: { name: "creator_profiles", type: "text" }
  - skill: "profile-validation"
    prompt: "validate-profile-depth"
    step_type: "validation"
    output: { name: "validation_result", type: "decision" }
  - skill: "profile-review"
    prompt: "review-profiles"
    step_type: "validation"
    gate: true
    output: { name: "review_feedback", type: "text" }
  - skill: "language-polish"
    prompt: "polish-language"
    step_type: "content"
    output: { name: "polished_profiles", type: "text" }
    context:
      voice_profile: "Neutral professional tone"
      grammar_strictness: "Professional"
    bindings:
      source:
        from_step: "Profile Assembly"
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

This workflow analyses a creator's writing and produces two structured deliverables: a **Voice Profile** (3,000-5,000 words of deep writing DNA) and an **Audience Profile** (who the content actually speaks to). Both profiles are reusable assets that personalise output across all content workflows.

The Voice Profile captures sentence-level patterns, vocabulary fingerprints, structural habits, rhetorical devices, and a personalised banned language list. Not "tone: casual" — specific, observable, reproducible patterns.

The Audience Profile identifies who the creator writes for based on what the content assumes, explains, and avoids. Not demographics — values, pain points, expertise levels, and repellents.

## Two Pathways

### Path 1: Content Samples (Recommended)

Provide 3-5 pieces of your writing — blog posts, newsletters, articles. More samples produce sharper profiles. The analyser extracts voice patterns, infers your audience, and calibrates the banned language list against your actual vocabulary.

### Path 2: Self-Description (No Samples)

If you don't have writing samples, describe your style in detail: how you start pieces, words you love and hate, who you write for, writers you admire. The analyser infers what it can and marks lower-confidence sections. Re-run with actual samples when available to sharpen the results.

## Pipeline Stages

### Stage 1: Voice Pattern Analysis

**Input:** Content samples or self-description, creator name, content type

Invoke the **voice-pattern-analysis** skill via the **analyse-voice-patterns** prompt. Performs deep linguistic analysis across seven dimensions: sentence architecture, paragraph rhythm, vocabulary fingerprint, rhetorical devices, structural habits, opinion style, and anti-patterns. Every finding cites direct quotes from the source material.

**Output:** Structured voice analysis with evidence quotes, preliminary voice markers, and confidence ratings.

### Stage 2: Audience Profile Synthesis

**Input:** Voice analysis from Stage 1, original content samples, creator context

Invoke the **audience-profile-synthesis** skill via the **extract-audience-signals** prompt. Infers the creator's audience from content signals: assumed knowledge, vocabulary level, problem framing, tone calibration, and engagement hooks. Compares inferred audience with creator's stated audience and flags any tension.

**Output:** Structured audience analysis with 2-3 reader segments, values, pain points, repellents, and tone calibration map.

### Stage 3: Banned Language Calibration

**Input:** Voice analysis, original content samples, default banned words reference

Invoke the **banned-language-calibration** skill via the **calibrate-banned-words** prompt. Scans content samples against the default banned list, removes words the creator genuinely uses, adds words they consistently avoid, and maps what the creator says instead of each banned term.

**Output:** Personalised banned words/phrases list with replacements and a caution list.

### Stage 4: Profile Assembly (Loop)

**Input:** All three analysis outputs, profile templates, validator feedback (on iterations 2+)

Invoke the **profile-assembly** skill via the **assemble-creator-profiles** prompt. Builds the complete Voice Profile (3,000-5,000 words) and Audience Profile using the **voice-profile-template** and **audience-profile-template** asset structures, filling every section with specific, evidence-backed content.

**Output:** Complete Voice Profile and Audience Profile.

### Stage 5: Profile Validation (Loop — Automated Check)

**Input:** Assembled profiles, original content samples

Invoke the **profile-validation** skill via the **validate-profile-depth** prompt. Evaluates profiles against seven checks: depth, specificity (10+ quotes), completeness, evidence grounding, banned list quality, voice profile length (3,000-5,000 words), and actionability. Returns a structured verdict with notes on each check.

**Output:** Structured verdict with per-check results and quality notes.

### Stage 6: Profile Review (Loop Verifier — Human Gate)

**Input:** Assembled profiles with validation notes

Execution **pauses** and presents the profiles to the creator for review. The creator reads the Voice Profile and Audience Profile, checks accuracy against their own understanding of their voice, and either approves or provides specific feedback.

- **If approved:** The loop exits and the profiles proceed to language-polish.
- **If feedback is provided:** The loop returns to Stage 4, where profile-assembly incorporates the creator's corrections into the next revision.

This is the verifier in the until_pass loop — the creator decides when the profiles are good enough, not the automated validator.

**Output:** Creator's approval or revision feedback.

### Stage 7: Language Polish

**Input:** Validated profiles

Invoke the **language-polish** skill to correct spelling, grammar, punctuation, and improve sentence clarity in the final profiles.

**Output:** Polished Voice Profile and Audience Profile.

### Stage 8: Consistency Check (Parallel)

**Input:** Polished profiles

Invoke the **consistency-check** skill to verify internal coherence — consistent terminology, no contradictions between sections, aligned evidence.

**Output:** Consistency report (supplementary — does not gate the main output).

## Error Handling

- If content samples are too short (under 300 words total), the analyser flags low confidence across all sections. Provide more content for better results.
- If the banned language calibration finds no words from the default list in the content, the personalised list may be unchanged — this is normal for creators who already avoid these patterns naturally.
- If the refinement loop reaches maxIterations (4) without the creator approving, the latest version of the profiles proceeds to language-polish. The profiles are still usable — the creator can re-run if further refinement is needed.
- If the self-description pathway produces very low confidence scores (1-2/5), the profiles include a recommendation to re-run with actual content samples.

## Outputs

| Name | Description |
|------|-------------|
| Voice Profile | 3,000-5,000 word analysis of the creator's writing DNA — sentence patterns, vocabulary, structure, anti-patterns, banned language, voice markers |
| Audience Profile | Reader segmentation, values, pain points, repellents, tone calibration map, content preferences |

## Inputs

| Name | Required | Description | Example |
|------|----------|-------------|---------|
| `{{input.content_samples}}` | Yes | 3-5 pieces of your writing, or a detailed description of your writing style | See Example Input below |
| `{{input.creator_name}}` | No | Your name or pen name for profile headers | `Alex Rivera` |
| `{{input.content_type}}` | No | What type of content you primarily produce | `Technical blog posts about software engineering` |
| `{{input.additional_context}}` | No | Anything you want the profiler to know — your audience, aspirations, pet peeves | `I write for senior engineers. I think I overuse parenthetical asides.` |

## Setup

Before running this workflow:

1. No external services required — paste your content directly.
2. For best results, prepare 3-5 pieces of your writing (500+ words each) covering different topics within your niche.
3. Review the **creator-profile-guide** document for tips on selecting the best content samples.

## Provider Notes

- All analysis steps (voice, audience, banned words) benefit from stronger models — they require close reading, evidence citation, and nuanced pattern detection.
- The profile-assembly step produces 3,000-5,000 words per iteration. Ensure your provider supports sufficient output length.
- The validation step runs JSON-structured evaluation — capable models handle this well.
- Total token usage is higher than typical workflows due to the refinement loop and long-form output. Budget for 4 iterations at 3,000-5,000 words each.

## Example Input

To test this workflow immediately after import:

```
Content Samples: |
  --- Sample 1: "Why Your Database Migrations Are Lying to You" ---

  Every time I hear "our migrations are idempotent," I know we're about to have a bad afternoon.

  Here's what actually happens. You write a migration that adds a column. Fine. You test it locally. It works. You deploy to staging. It works. You deploy to production at 2am because your team still thinks that's somehow safer, and the migration locks a 400-million-row table for eleven minutes.

  Nobody tested it against a table that size. Nobody asked "what does ALTER TABLE do under the hood on Postgres 15 with this many rows?" Nobody checked whether the default value triggers a full table rewrite.

  I've done this. I've been the person staring at pg_stat_activity wondering why every query is waiting on AccessExclusiveLock. It's not fun. It's also completely preventable.

  The fix isn't better tooling. The fix is treating migrations as production operations, not developer conveniences. That means:

  - Load-testing migrations against production-scale data (not your 50-row dev database)
  - Running them during business hours with a rollback plan, not at 2am when half the team is asleep
  - Separating schema changes from deploy pipelines entirely

  "But that's slow." Yes. Slower than an eleven-minute outage? Probably not.

  --- Sample 2: "Stop Calling Everything Technical Debt" ---

  Technical debt has become the engineering equivalent of "we need to talk." It means something bad happened, someone is frustrated, and the proposed solution involves stopping feature work for an unspecified period.

  The problem isn't technical debt. The problem is that we use one term to describe at least four completely different situations:

  1. **Deliberate shortcuts** — we knew this was wrong, we shipped it anyway, we have a ticket to fix it. This is actual debt. It has a known principal and a predictable interest rate.

  2. **Accumulated cruft** — nobody made a bad decision. The codebase grew, requirements shifted, and what was clean three years ago is now a mess. This isn't debt. It's entropy.

  3. **Missing capabilities** — we don't have CI/CD, monitoring, or proper testing infrastructure. This isn't debt either. It's underinvestment. You can't pay down a debt you never took on.

  4. **Skill gaps** — the code is bad because the team at the time didn't know better. Not their fault. But calling it "debt" implies someone borrowed something. They didn't. They built what they could with what they knew.

  When you lump all four into "tech debt" and present it to leadership as a single bucket that needs "20% of sprint capacity," you get blank stares. Rightly so. You're asking for a blank cheque to fix an undefined problem.

  Instead: name the specific thing. "Our user table migration path doesn't support zero-downtime deploys. Fixing it takes 3 sprints and eliminates our quarterly deployment freeze." That's a conversation leadership can have.

  --- Sample 3: "The Oncall Engineer's Survival Guide" ---

  Your first oncall shift will go one of two ways. Either nothing happens and you spend a week with your phone volume at maximum feeling vaguely anxious, or everything happens at once and you learn more about your system in 72 hours than you did in your first six months.

  Both are normal. Neither prepares you for the third option, which is the one nobody warns you about: the slow drip. One alert every few hours. None of them critical. All of them just ambiguous enough that you can't ignore them but can't confidently resolve them either. By Wednesday you're exhausted and you haven't had a single major incident.

  Here's what I wish someone told me before my first shift.

  **The runbook is a starting point, not a script.** If the runbook says "restart the service" and restarting doesn't fix it, the runbook has failed. Your job is now to diagnose, not to follow steps. Write down what you find so the next person has a better runbook.

  **Escalation is not failure.** The entire point of having a team is that no single person knows everything. If you've spent 15 minutes and you're not making progress, pull someone in. The 15-minute rule exists because after 15 minutes of solo debugging at 3am, your judgement is already degrading.

  **Write the postmortem while it's fresh.** Not tomorrow. Not next week. Now. The details you think you'll remember — the exact error message, the sequence of things you tried, the thing that actually fixed it — will be gone by Monday.

Creator Name: "Alex Rivera"
Content Type: "Technical blog posts about software engineering practices"
Additional Context: "I write for senior engineers and engineering managers. I think my style is fairly direct but I haven't mapped it properly. I want to use this profile to keep my voice consistent when I start using AI to help draft posts."
```
