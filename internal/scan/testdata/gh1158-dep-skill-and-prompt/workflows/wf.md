---
type: workflow
id: wf
title: "Dep Skill And Prompt Workflow"
description: "The live catalogue shape: the declaring skill and the declaring prompt are both shared"
connections:
  - target: shared-skill
    type: uses
inputs:
  tone:
    type: text
    required: true
    description: "the tone to write in"
execution:
  - skill: "shared-skill"
    prompt: "shared-prompt"
    step_type: "generation"
    bindings:
      tone:
        from_input: "tone"
---

## Overview

Both declaration sites arrive through the resolver. Whether this passes depends
entirely on whether the two sites AGREE.
