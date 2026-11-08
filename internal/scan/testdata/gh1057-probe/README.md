# GH#1057 probe — SYNTHETIC, and it is the artefact, not a description of one

Written here (not copied from Hub): no published package authors this shape
deliberately, which is the point — it is a probe for three engine behaviours
that were invisible because nothing refused them.

`duplicate-skill-probe/` ships **four execution entries that produce four
steps**:

| # | Entry | What the engine does with it |
|---|---|---|
| 1 | `skill: alpha-skill` + `prompt: outline-alpha` | its **own step** at position 0, carrying `outline-alpha`'s body and declaring output `outline` |
| 2 | `prompt: loner-prompt` (prompt-only) | runs **last** — the execution loop assigns positions only to `skill:` / `id:`+`local.builtin` / `action:` entries, so this keeps the default 999 |
| 3 | `skill: alpha-skill` + `prompt: draft-alpha` | its **own step** at position 1, carrying `draft-alpha`'s body and declaring output `draft` — **same `Alpha Skill` title as entry 1** |
| 4 | `skill: beta-skill` + `prompt: beta-prompt` | ordinary |

> **This table used to read "four entries, three steps".** Until app GH#1092
> ("an execution entry is a STEP, and the engine stops choosing silently"),
> `ResolveStepPrompts` keyed `skill -> prompt` by skill slug and the planner
> resolved every `skill:` entry to the same `PlannedStep`, so entry 3 overwrote
> entry 1 and `outline-alpha` never ran. Both entries now plan their own step
> and both prompts run. **The refusal below did not change**, because it never
> rested on the overwrite — see assertion 1.

**What it pins, all three at once:**

1. **`workflow.duplicate_skill_entry` fires on entries 1+3** (GH#1057's ruling,
   upheld verbatim by GH#1092: the skill is the addressable unit, because
   `from_step` binds by skill TITLE, so two steps on one skill share a title and
   nothing downstream can say which one it means). GH#1092's own words:
   *"same-skill-twice is now EXPRESSIBLE, not supported ... plan-time
   expressible, publish-time refused."* This probe is the publish-time half.
2. **`workflow.node_never_invoked` stays SILENT on `outline-alpha`** — it is
   *named* in the execution block, and that rule tests naming, not
   addressability. This fixture is the standing record of that blind spot and
   the reason the duplicate clause is the thing that closes it.
3. **The prompt-only step still sorts last** — authored second, planned fourth.
   That is an authoring-surface gap (Hub emits no connection `position:`), owned
   elsewhere; asserted here so a future fix is visible as a fixture change
   rather than as a silent reorder.

Keep the authored order as it is. Entry 2 sitting **second** is load-bearing for
assertion 3 — move it last and the fixture stops proving anything.
