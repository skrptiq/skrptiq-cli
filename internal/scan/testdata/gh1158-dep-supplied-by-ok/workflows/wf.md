---
type: workflow
id: wf
title: "Dep Supplied-By Conforming Workflow"
description: "Binds the declared slot from an INPUT, which is what the slot says it takes"
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
    prompt: "tone-prompt"
    step_type: "generation"
    bindings:
      tone:
        from_input: "tone"
---

## Overview

The same `tone` slot, bound from an input. Nothing here should be refused.
