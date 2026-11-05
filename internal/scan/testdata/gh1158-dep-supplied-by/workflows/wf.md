---
type: workflow
id: wf
title: "Dep Supplied-By Workflow"
description: "Step 2 uses a DEP-PROVIDED skill whose context_params declare supplied_by"
connections:
  - target: local-skill
    type: uses
  - target: shared-skill
    type: uses
execution:
  - skill: "local-skill"
    prompt: "first-prompt"
    step_type: "generation"
    output: { name: "first_out", type: "text" }
  - skill: "shared-skill"
    prompt: "tone-prompt"
    step_type: "generation"
    bindings:
      tone:
        from_step: "Local Skill"
        field: output
---

## Overview

The `tone` slot is declared `supplied_by: input` on the DEP-PROVIDED
`shared-skill`, and bound here from a STEP. The refusal can only fire if the
declaration survives the resolver.
