# GH#1194 corpus fixtures — the two REAL published packages the naive rule refuses

Verbatim copies from `skrptiq-hub` `main`, not synthetic cases. Both contain the
string `local.shell` and **neither executes a shell step**: the occurrences are
PROSE in a prompt listing the valid step-type vocabulary —

> `local.builtin`, `local.shell`, `local.http`, `local.template`, …

A text-level or file-grep rule refuses the two packages whose purpose is to
**document** the step types. `skrptiq-on-skrptiq` is also the crossing demo's own
home (R8), so the naive rule would refuse the package the MVP is built in.

**Both carry a `manifest.id`**, which is load-bearing: the rule is therefore
ACTIVE on them. A copy without the id would pass the test through the local-only
silence path and prove nothing.

This is the population floor applied to a gate's negative direction. GH#858's
34 false refusals across 10 working packages were found by the corpus and by
nothing else.
