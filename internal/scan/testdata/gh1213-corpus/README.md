# GH#1213 corpus fixtures — the false positive this issue's wording invites

Verbatim from `skrptiq-hub` `origin/main`. Both contain **`type: file`** and
**neither declares an output destination**: every occurrence is an `inputs:`
declaration —

```yaml
inputs:
  study_material:
    label: "Study Material"
    required: true
    type: file
```

`type: file` appears in **28 published files** and not one is a destination. A
rule keyed on that string would refuse 28 working packages, so the clause reads
the `outputs` key and never the text.

Same lesson as GH#858's 34 false refusals and GH#1194's two documentation
packages: the negative direction is what the corpus is for.
