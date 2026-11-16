# GH#1253 corpus fixtures — the fenced-reference false refusal

Verbatim from `skrptiq-hub` `origin/main`.

| package | inputs | before | after |
|---|---|---|---|
| `documentation-pipeline` | `source_code` | **WARNING, exit 1** | clean, exit 0 |
| `creator-profile-builder` | `content_type`, `creator_name` | **WARNING ×2, exit 1** | clean, exit 0 |
| `weekly-report` | — | clean | clean *(control)* |

## Why they warned

`prompts/analyse-code.md` in `documentation-pipeline`:

```
38:  **Language:** {{input.language}}      <- prose, found
40:  ```                                   <- fence opens
41:  {{input.source_code}}                 <- INSIDE the fence, stripped before detection
42:  ```
```

`stripCode` removes fenced blocks, so `source_code` was never in the text Rule 3
searched — while `language`, identical in declaration and three lines away, was.
Causation was established by moving the reference out of the fence and changing
nothing else: exit 1 became exit 0.

**It is the natural case**, which is what made it bad: a prompt that feeds *code*
to a model puts the placeholder in a fence.

## Why `weekly-report` is here

It is the control for the *other* direction. Its prompt writes load-bearing
references inside **inline backticks** — "if `{{input.week_start}}` is provided" —
and the `fencedBlock` comment records that an earlier draft which stripped inline
spans too falsely refused its two correctly-declared inputs. It must stay clean:
a fix that widened the strip in the wrong place would show up here.

These were the **only** two packages in the whole catalogue with any
`input_declared_unused` warning, so all three warnings were false positives.
