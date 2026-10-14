---
type: skill
id: summary-skill
title: "Summary Skill"
description: "A second valid test skill, so each prompt has a skill of its own"
connections:
  - target: spaced-ref-prompt
    type: derived_from
---

This skill exists because a skill is the addressable unit: two execution entries
naming one skill collapse to a single step (GH#1057), so the spaced-reference
prompt needs its own skill to be reachable.
