# GH#975 corpus fixtures — COPIES, do not edit to make a test pass

Copied from `skrptiq-hub/examples` on **2026-10-12**:

| Here | Source package |
|---|---|
| `weekly-report/` | `skrptiq-hub/examples/weekly-report` (verbatim) |
| `meta-authoring/prompts/validate-structure.md` | `skrptiq-hub/examples/skrpt-builder` |
| `meta-authoring/prompts/validate-build.md` | `skrptiq-hub/examples/skrptiq-on-skrptiq` |
| `meta-authoring/skills/structure-validation.md` | `skrptiq-hub/examples/skrpt-builder` |
| `meta-authoring/skrptiq.yaml`, `workflows/` | written here, to execute the copied nodes |

**Why copies and not a path into the sibling repo:** a test that breaks when
someone edits Hub content is a test that gets deleted by whoever is
inconvenienced by it. The cost is staleness, which the date above makes visible.

**What they pin.** `weekly-report` is the package in GH#975's title, and it is
CORRECT: both inputs carry `required: false` in `prompts/fetch-week-data.md`. If
a change to the input rules ever rejects it, the rule is wrong, not the content —
that is the assertion. The `meta-authoring` files are the real prose that
documents the rule being enforced ("every `{{input.VAR}}` variable in prompt
bodies has a matching `inputs` entry"), which a gate must never refuse.
