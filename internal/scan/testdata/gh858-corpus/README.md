# GH#858 corpus fixtures — COPIES, do not edit to make a test pass

Copied verbatim from `skrptiq-hub/examples` on **2026-10-13**:

| Here | Source package | What it pins |
|---|---|---|
| `morning-briefing/` | `skrptiq-hub/examples/morning-briefing` | **A working package the first draft of this rule falsely refused.** Its first two steps sit inside an `execution:` `- parallel:` group, which nests them one level deeper. Reading only each entry's top-level `skill:`/`prompt:` reported all four of its nodes as never invoked — 34 false refusals across 10 packages, this being the simplest shape of them. Must scan free of `workflow.node_never_invoked`. |
| `status-report-flow/` | `skrptiq-hub/examples/status-report-flow` | **A genuine orphan in published content.** `prompts/stakeholder-update-email.md` ships, the workflow body says *"Invoke the **stakeholder-update-email** prompt to produce a polished, ready-to-send status update email"*, and the execution block never names it. Must be refused, naming that node. |

**Why copies and not a path into the sibling repo:** a test that breaks when
someone edits Hub content is a test that gets deleted by whoever it inconveniences.
The cost is staleness, which the date above makes visible.

**`status-report-flow` is expected to be FIXED in Hub eventually.** When it is, this
copy keeps the refusal assertion meaningful — it is a frozen specimen of the defect,
not a live mirror. If the real package is repaired, do not "update" this copy to
match; that would delete the only test of the refusal path.
