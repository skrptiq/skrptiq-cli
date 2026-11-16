# GH#1131 corpus fixtures — two live false refusals and the control that proves a fix is a fix

Verbatim from `skrptiq-hub` `origin/main`.

| package | input | before this change |
|---|---|---|
| `batch-competitor-analysis` | `competitors` | **WARNING `input_declared_unused`, exit 1** |
| `profile-social-pipeline` | `platforms` | **WARNING `input_declared_unused`, exit 1** |
| `translation-pipeline` | `target_languages` | **clean, exit 0** |

Both warnings were false: each input is consumed by
`inputExpression: "{{input.<name>}}"`, which Rule 3 could not see because its walk
keyed on `metadata["execution"]` and `loops` is a sibling key.

Under hub's `caller-release.yml` policy — *"any engine finding blocks — errors
and warnings alike"* — **those two packages could not be published.**

**`translation-pipeline` is the control and it is not synthetic.** It declares the
same kind of loop over an input, and it passed *before* the fix because that
input ALSO appears in a prompt body (`prompts/translation-brief.md`), which Rule 3
does read. It must stay clean afterwards: a change that merely switched Rule 3
off would look identical to a fix on the other two and only this one would catch
it.

Corpus rather than synthetic per the GH#1213 constraint — the 28 `type: file`
false-positive candidates are why.
