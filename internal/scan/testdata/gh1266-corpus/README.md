# GH#1266 — Rule 1 was switched off package-wide by one straddling execution entry

`weekly-report-split` is the real hub package `weekly-report` with ONE change:
`week_start`'s declaration is deleted from `prompts/fetch-week-data.md`. Its
`## Inputs` table row in the workflow (documenting it) and its body reference
(`prompts/fetch-week-data.md`, "if `{{input.week_start}}` is provided") are both
left intact.

That is a textbook Anti-Pattern 5: documented, referenced, declared nowhere. It
should be an ERROR by Rule 1's promotion ladder.

**Before GH#1266 this package scanned clean — exit 0, zero findings.** Not a
missed promotion: Rule 1 did not run at all. One execution entry
(`skill: synthesise-weekly-report`, a dependency, paired with the local
`prompt: synthesise-report`) set a package-wide `splitAcrossDep` flag, and that
flag skipped EVERY Rule 1 finding for all four steps.

It was measured at **42 of 88 workflow-bearing catalogue packages — 48%** — and
it was found by running a positive control that was checking something else.

The suppression itself is correct and still exists, narrowed to the nodes of the
straddling entry: a dependency's node content is served with its frontmatter
stripped (203 dep nodes cached, 87 with content, **0** carrying an `inputs:`
block), so a declaration on the dep half is genuinely unreadable.
