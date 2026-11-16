---
type: prompt
id: calibrate-banned-words
title: "Calibrate Banned Words"
description: "Personalise the default banned words list against the creator's actual vocabulary usage"
tags: [Production, Analysis, Quality, Content]
connections:
  - target: banned-language-calibration
    type: derived_from
metadata:
  output_format: markdown
  prompt_type: task
---

You are a vocabulary calibration specialist. Your job is to personalise the default banned language list for a specific creator.

## Default Banned Language Reference

The following categories of words and phrases are banned by default because they signal generic AI-generated content. Your job is to check each one against this creator's actual usage.

### AI Cliché Words
delve, dive (into), unlock, unleash, harness, leverage (as verb), streamline, optimize, empower, elevate, robust, seamless, transformative, pivotal, nuanced, holistic, synergy, paradigm, landscape (metaphorical), tapestry, intricate, utilize, facilitate, endeavour, myriad, plethora, realm, foster, bolster, cornerstone

### AI Cliché Phrases
"In today's [fast-paced/digital/modern] world", "It's worth noting that", "At the end of the day", "When it comes to", "In order to", "The fact that", "It goes without saying", "Needless to say", "As we all know", "In conclusion", "Let's dive in", "Without further ado", "Game-changer", "Paradigm shift", "Best practices", "Take it to the next level", "Think outside the box", "Move the needle", "Low-hanging fruit", "Circle back", "Touch base", "Seamless integration", "Enterprise-grade", "Industry-leading", "Cutting-edge", "Breakthrough", "Revolutionary"

### Empty Intensifiers
very, really, extremely, incredibly, absolutely, literally (figurative), essentially, fundamentally, basically

### Hedging Crutches
"I think" (redundant in opinion pieces), "In my opinion", "It could be argued that", "Some might say", "Perhaps", "Arguably"

## Calibration Process

### Step 1: Scan for Matches

Search the creator's content samples for every word and phrase in the lists above. For each match found:
- Record the exact context (the sentence containing the word)
- Assess whether the usage is natural and intentional or incidental
- Count occurrences across all samples

### Step 2: Classify Each Entry

For every item in the default banned list, assign one of:

- **BAN** — Does not appear in any sample, or appears only in contexts that suggest it was unintentional. Keep on the banned list.
- **REMOVE** — Appears naturally and intentionally in the creator's writing. Remove from their personalised ban list with a note explaining why.
- **CAUTION** — Appears 1-2 times across all samples. Flag for review but don't auto-reject.

### Step 3: Detect Additional Avoidances

Identify common words and phrases in the creator's genre that never appear in their samples despite being expected. These are candidates for addition to the personalised ban list.

Look for:
- Common transitions they avoid
- Industry buzzwords they never use
- Intensifiers they skip
- Opening/closing formulas they reject

### Step 4: Map Replacements

For each BANNED entry, identify what the creator says instead. Look for:
- Direct synonyms they prefer
- Structural alternatives (they avoid the word by restructuring the sentence)
- Omission (they simply never express that concept)

## Output Format

```
# Banned Language Calibration: [Creator Name]

## Scan Summary
- Default list entries checked: [N]
- Found in content: [N] (removed or cautioned)
- Not found (banned): [N]
- New additions: [N]

## Personalised Ban List

### Banned Words
| Word | Status | Evidence | Replacement |
|------|--------|----------|-------------|
| delve | BANNED | Not found in any sample | [what they say instead, or "N/A — concept not expressed"] |
| leverage | REMOVED | Found 3x in financial context — legitimate domain usage | N/A |
| ... | ... | ... | ... |

### Banned Phrases
| Phrase | Status | Evidence | Replacement |
|--------|--------|----------|-------------|
| "In today's world" | BANNED | Not found | Creator opens with direct claims |
| ... | ... | ... | ... |

### Caution List
| Word/Phrase | Occurrences | Context | Recommendation |
|-------------|-------------|---------|----------------|
| essentially | 1 in 5 samples | Used when simplifying a technical concept | Flag if used more than once per piece |

### Additional Bans (Creator-Specific)
| Word/Phrase | Reason | Genre Frequency |
|-------------|--------|-----------------|
| [word common in genre but absent from all samples] | Never used despite being common in [genre] | High |

## Calibration Notes
[Any patterns worth noting — e.g. "Creator avoids ALL intensifiers, not just the default list" or "Creator uses informal register throughout, so formal hedging crutches are irrelevant"]
```

{{steps.previous.output}}
