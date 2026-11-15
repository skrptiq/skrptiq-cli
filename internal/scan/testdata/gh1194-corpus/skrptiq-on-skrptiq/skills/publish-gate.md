---
type: skill
id: publish-gate
title: Publish Gate
description: "Human gate — pauses for the user to review the publication readiness report and approve for Hub publication"
tags: [Production, Gate, Stage-4]
connections:
  - target: llm-service
    type: runs_on
---

## Capability

Pauses the workflow and presents the publication readiness report to the user. The user reviews the results and either approves for publication or decides to revise.

This is the final gate in the pipeline. After approval, the skrpt is ready to save as a workspace and publish to the Hub.

## What Happens

1. Execution pauses with status `awaiting_input`
2. The user sees the publication readiness report: deterministic check results, semantic check results, overall verdict, issues list
3. The user responds with approval or a decision to revise
4. If approved, the pipeline is complete — the generated files are publication-ready

## Review Guidance

- Are all deterministic checks passing?
- Are the semantic warnings acceptable or do they need addressing?
- Is the overall verdict "Ready" or "Ready with warnings"?
- If "Not ready", which issues are blocking?

## How to Respond

- **To approve:** Type "approved" — the skrpt is publication-ready. Save the generated files from Stage 3 as a workspace.
- **To revise:** Note what needs changing — you'll need to return to Stage 3 (Skrpt Build) and run it again with your feedback.
- **To stop:** Type "stop" if the skrpt needs fundamental rework.
