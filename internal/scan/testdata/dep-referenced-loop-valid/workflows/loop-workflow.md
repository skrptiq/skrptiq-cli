---
type: workflow
id: loop-workflow
title: "Loop Workflow"
description: "until_pass loop whose steps reference dep-provided slugs"
metadata:
  loops:
    - id: "review-loop"
      mode: "until_pass"
      steps:
        - "dep-skill"
        - "dep-prompt"
      verifier: "dep-prompt"
      maxIterations: 3
---

`dep-skill` and `dep-prompt` resolve via hub-shared/test-dep.
The loop's steps must not error under three-tier resolution.
