---
type: workflow
id: meta-authoring
title: Meta Authoring
description: "Executes the copied teaching prompts"
execution:
  - skill: "structure-validation"
    step_type: "generation"
    prompt: "validate-structure"
  - skill: "structure-validation"
    step_type: "generation"
    prompt: "validate-build"
---

## Overview

Fixture only. The prompts it runs are verbatim copies of published content whose
prose contains literal `{{input.VAR}}` / `{{input.field_name}}` examples.
