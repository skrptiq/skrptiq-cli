---
type: skill
id: profile-validation
title: Profile Validation
description: "Quality gate for assembled profiles — checks depth, specificity, evidence grounding, and completeness"
tags: [Production, Validation, Quality, Loop]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Evaluates the assembled Voice Profile and Audience Profile against seven quality criteria. Acts as the verifier in the until_pass refinement loop — if any check fails, it provides specific feedback for the profile-assembly step to address in the next iteration.

## When to Use

- Immediately after profile-assembly in the refinement loop
- Runs on every iteration until it passes or maxIterations is reached

## Validation Criteria

### 1. Depth
Does the Voice Profile contain sentence-level patterns, not just adjective labels?

- **Fail:** "The creator has a conversational tone" (surface label)
- **Pass:** "Opens 60% of pieces with a provocative question, follows with 'Here's what actually happens', uses 8-12 word sentences for claims then 25-30 word sentences for evidence" (specific patterns)

### 2. Specificity
Are at least 10 concrete examples quoted from the original content?

- Count direct quotes across both profiles
- Each quote must be attributed to a specific sample
- Generic examples ("the creator might write something like...") do not count

### 3. Completeness
Are all template sections filled? No placeholders, no "N/A", no generic filler?

- Every section in both templates must have substantive content
- Sections marked "if applicable" may be omitted with justification

### 4. Evidence Grounding
Do the claims match what the content samples actually show?

- Spot-check 5+ claims against the source material
- Flag any claim that contradicts the evidence or extrapolates beyond it

### 5. Banned List Quality
Is the banned list calibrated, not the default list copy-pasted?

- At least 3 entries must be removed or modified based on creator usage
- Replacement suggestions must reflect what the creator actually says
- Caution list must exist and be distinct from the ban list

### 6. Voice Profile Length
Is the Voice Profile within the 3,000-5,000 word target?

- Under 2,500 words: fail (insufficient depth)
- 2,500-3,000 words: continue (needs more depth in specific sections)
- 3,000-5,000 words: pass
- Over 5,000 words: continue (needs tightening — remove repetition, not substance)

### 7. Actionability
Could another AI or human writer use these profiles to write convincingly in this creator's voice?

- The Voice Markers section must contain 5-10 ranked features
- The Usage Guide section must include concrete instructions
- The Audience Profile's Tone Calibration Map must be specific enough to configure a writing prompt

## Output Format

Returns a structured verdict:

```json
{
  "status": "pass | continue | fail",
  "checks": {
    "depth": { "result": "pass|continue|fail", "notes": "..." },
    "specificity": { "result": "pass|continue|fail", "notes": "...", "quote_count": 0 },
    "completeness": { "result": "pass|continue|fail", "notes": "..." },
    "evidence_grounding": { "result": "pass|continue|fail", "notes": "..." },
    "banned_list_quality": { "result": "pass|continue|fail", "notes": "..." },
    "voice_profile_length": { "result": "pass|continue|fail", "notes": "...", "word_count": 0 },
    "actionability": { "result": "pass|continue|fail", "notes": "..." }
  },
  "overall_notes": "...",
  "instructions": "Specific feedback for the next iteration — which sections need work and what to fix."
}
```

## Pass Criteria

- **pass**: All 7 checks pass
- **continue**: 1-3 checks fail — provide specific revision instructions
- **fail**: 4+ checks fail — fundamental issues, but still attempt revision

On iteration 3+, the validator should be slightly more generous on marginal checks to avoid burning tokens on diminishing returns.
