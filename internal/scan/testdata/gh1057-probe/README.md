# GH#1057 probe — SYNTHETIC, and it is the artefact, not a description of one

Written here (not copied from Hub): no published package authors this shape
deliberately, which is the point — it is a probe for three engine behaviours
that were invisible because nothing refused them.

`duplicate-skill-probe/` ships **four execution entries that produce three
steps**:

| # | Entry | What the engine does with it |
|---|---|---|
| 1 | `skill: alpha-skill` + `prompt: outline-alpha` | **collapses** — `ResolveStepPrompts` keys `skill -> prompt` by skill slug, so entry 3 overwrites this one and `outline-alpha` NEVER RUNS |
| 2 | `prompt: loner-prompt` (prompt-only) | runs **last** — the execution loop assigns positions only to `skill:` / `id:`+`local.builtin` / `action:` entries, so this keeps the default 999 |
| 3 | `skill: alpha-skill` + `prompt: draft-alpha` | the step the collapse keeps |
| 4 | `skill: beta-skill` + `prompt: beta-prompt` | ordinary |

**What it pins, all three at once:**

1. **`workflow.duplicate_skill_entry` fires on entries 1+3** (GH#1057's ruling:
   the skill is the addressable unit, because `from_step` binds by skill TITLE,
   so a second step per skill would make every such binding silently ambiguous).
2. **`workflow.node_never_invoked` stays SILENT on `outline-alpha`** — it is
   *named* in the execution block, and that rule tests naming, not
   reachability. This fixture is the standing record of that blind spot and the
   reason the duplicate clause is the thing that closes it.
3. **The prompt-only step still sorts last** — authored second, planned third.
   That is an authoring-surface gap (Hub emits no connection `position:`), owned
   elsewhere; asserted here so a future fix is visible as a fixture change
   rather than as a silent reorder.

Keep the authored order as it is. Entry 2 sitting **second** is load-bearing for
assertion 3 — move it last and the fixture stops proving anything.
