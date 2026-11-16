---
type: prompt
id: take-notes
title: Take Notes
description: "Core prompt for structuring study notes"
tags: [Production, Learning, Research]
context_params:
  source:
    label: "Prior Analysis"
    description: "The prior Analysis supplied by the upstream step."
    required: true
inputs:
  module_name:
    label: "Module Name"
    description: "The name of the course module or subject"
    example: "Introduction to Macroeconomics"
    required: true
    type: text
  topic:
    label: "Topic"
    description: "The main subject or topic to address"
    example: "The impact of remote work on team productivity"
    required: true
    type: text
  study_material:
    label: "Study Material"
    description: "The material to study — lecture notes, textbook chapters, or readings"
    example: "[Paste lecture notes or reading material here]"
    required: true
    type: file
    accept: ".pdf,.txt,.md,.docx"
---

## Purpose

Transforms raw study material into structured, retrievable notes using the Cornell method.

## Prompt

You are a study skills tutor. Create structured notes from the following study material for the module {{input.module_name}} on the topic {{input.topic}}.

**Study material:**
{{input.study_material}}

### Prior Analysis

{{step.context.source}}

Create structured notes using the Cornell method. Organize the content into: 1) Main notes section with key concepts, definitions, and explanations. 2) Cue column with questions and keywords for self-testing. 3) Summary section capturing the essential points in 3-5 sentences. Highlight any concepts that would benefit from further reading.
