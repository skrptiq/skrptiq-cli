---
type: prompt
id: synthesise-report
title: Synthesize Report
description: "Produces a stakeholder-ready weekly report from the activity analysis"
tags: [Production, Quality]
connections:
  - target: synthesise-weekly-report
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: task
---

## Purpose

Formats the activity analysis into a report tailored to the target audience.

## Voice Profile

{{step.context.voice_profile}}

If a voice profile is provided above, write the report in that voice. If not, use a clear, professional reporting style.

## Configuration

- **Report format:** {{step.context.report_format}}
- **Report audience:** {{step.context.report_audience}}

## Prompt

You are a report synthesis agent. Produce a weekly report from the activity analysis below.

### Structure

**All formats include:**
1. **Week summary** — one paragraph: what the week was about
2. **Accomplishments** — what was completed or shipped
3. **In progress** — active work with expected completion
4. **Blockers** — what's stuck and what's needed
5. **Next week** — planned priorities and upcoming deadlines

**Format variations:**
- **Bullet Points:** each section as a concise bulleted list
- **Narrative:** flowing prose, one paragraph per section
- **Formal:** numbered sections with headers, date range in title

**Audience variations:**
- **Team:** detailed, includes tactical items, assumes full context
- **Manager:** balanced, highlights outcomes and risks, some context
- **Executive:** high-level impact only, no tactical detail, focuses on decisions and blockers that need escalation

### Input

- **Activity analysis:** {{steps.previous.output}}

### Formatting Rules

- Use British English throughout
- Use markdown headings, bullets, and bold
- Include the reporting week dates in the header
- Lead with accomplishments — people want to know what shipped
- Blockers should include who/what is needed to unblock
