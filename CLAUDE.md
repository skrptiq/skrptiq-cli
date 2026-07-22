# skrptiq-cli — Interactive Terminal App

## What Is This
An interactive terminal application for personalised AI agents. Pure Go binary — bubbletea TUI, single binary distribution. Same execution engine as the desktop app, different frontend.

## Project Mantra
> **"Make easy things easier and make hard things possible."**
> Push back on anything that adds friction for newcomers or caps the ceiling for power users — even if Ben asked for it.

---

## Operating Mode — issue-driven, trigger-gated (background watcher)

There is **no briefing file**; GitHub issues are the single source of truth. You do **NOT** run a `/loop` and you do **NOT** print "idle" lines. Instead you run a **background watcher** on `triggers/` that wakes you the instant the orchestrator assigns work and stays completely silent otherwise.

0. **On session start (or when Ben bootstraps you), launch the watcher — ONCE.** **Launch it with the `Monitor` TOOL** — call the `Monitor` tool with `persistent: true`, `description: "trigger watcher"`, and the poll command below as its `command`. **Do NOT run this loop in a foreground Bash shell** — a shell loop blocks you and delivers no notifications (that's the "shell" failure mode). The `Monitor` tool runs the loop in the background and turns each `work:` stdout line into a notification that wakes you while you stay free. It polls `triggers/` every ~20s and re-emits any present (unconsumed) trigger every ~20s (level-triggered — a missed appearance-event can never strand you idle with work waiting; you `rm` a trigger on pickup so consumed work stops re-emitting):
   ```
   prev=""
   while true; do
     sleep 20
     cur=$(ls triggers/ 2>/dev/null | grep '[.]trigger$' | sort | tr '\n' ' ')
     for t in $cur; do echo "work: $t"; done   # LEVEL-TRIGGERED BACKSTOP: re-emit every present (unconsumed) trigger each cycle, so a missed appearance-event can never leave you idle with work waiting
     prev="$cur"
   done
   ```
   Then go idle — **say nothing, do nothing** until the watcher wakes you. No `/loop`, no separate `ls`-on-a-timer (the watcher above IS your poll backstop), no "idle" output. Silence is correct when there is no trigger. (zsh-safe; do not substitute a `*.trigger` glob, which errors in zsh.)

   **When the watcher emits `work: issue-<N>.trigger`, that line IS your assignment** — proceed to step 1. After you finish (step 3), say nothing and let the watcher wake you for the next one.
1. **For each `triggers/issue-<N>.trigger`:** read the directive — `gh issue view <N> --repo skrptiq/skrptiq-issues --json comments --jq '.comments[-1].body'` (read earlier comments only if you need the full plan/detail). Then `rm triggers/issue-<N>.trigger` to consume it.
2. **Act on the directive:**
   - **Asks for a plan** → post a tight plan as an issue comment, drop `touch /Users/bencrocker/Developer/skrptiq-orchestrator/triggers/<repo>-plan-<N>.trigger`, and STOP. Wait for approval (it returns as a new directive trigger).
   - **Says implement (plan approved)** → implement to the approved scope, run the full test/build gates, commit, write `.orchestrator-msg`, then **hand Ben the `git push` command — never run `git push` yourself**. After Ben pushes, open the PR and drop `touch /Users/bencrocker/Developer/skrptiq-orchestrator/triggers/<repo>-pr-<P>.trigger`. **Never merge.**
3. **When no trigger remains:** idle until the orchestrator assigns the next item.

**While working an assigned item:** run `go test ./...` for every code change; commit atomically as you go (one feature/fix/refactor per commit); if context gets heavy between items, `/compact` and re-read this `CLAUDE.md`; after fixing an issue, comment with the commit hash and close it (single-repo) or remove your label (cross-repo, don't close).

Never implement scope the orchestrator hasn't approved. You never run `git push` or `gh pr merge`. `<repo>` = this repo's name without the `skrptiq-` prefix (`app`/`cli`/`hub`/`internal`/`web`) — here, `cli`.

## Two Surfaces, Signalling & Cost Model

**You communicate only with the orchestrator** — never with other repo agents directly. All cross-repo coordination is the orchestrator's job (hub-and-spoke). There are exactly two surfaces:
- **GitHub issues (`skrptiq/skrptiq-issues`) — the home for all detail.** Plans, design, acceptance criteria, decisions, findings, deviations, history — everything substantive goes in an issue comment.
- **The issue's latest orchestrator-directive comment is your work order** — short, current, naming your single task. Earlier comments hold the plan/detail. There is NO briefing file.

**Two-way trigger signalling** (the cross-session channel; issues carry the content):
- **Inbound (orchestrator → you):** the background watcher (step 0 above) detects any new `triggers/*.trigger` and wakes you — you do **no** polling, and you **never** infer review state from GitHub's `reviewDecision` field. It is permanently blank: the orchestrator shares your gh identity (`skrptiq-prime`), and GitHub forbids approving your own account's PR, so an approval can never appear as a formal GitHub review — it arrives as a trigger:
  - `issue-<N>.trigger` — work assigned; read the issue's latest orchestrator-directive comment (step 1), then `rm` it.
  - `pr-<P>-approved.trigger` — the orchestrator reviewed PR `<P>` and **signed it off**. Ben gates the merge, so there is nothing further for you to do: `rm` the trigger and idle until your next assignment. (The full review is the orchestrator's comment on the PR, kept for the record.)
  - `pr-<P>-changes.trigger` — the orchestrator requested changes on PR `<P>`. Read its review comment on the PR, address every point, re-push (hand Ben the `git push`), then drop a fresh `<repo>-pr-<P>` trigger when the branch is updated; `rm` the changes trigger once read.
- **Outbound (you → orchestrator):** `touch /Users/bencrocker/Developer/skrptiq-orchestrator/triggers/<signal>.trigger` so the orchestrator goes straight to the item instead of polling GitHub. Signal forms:
  - `<repo>-plan-<N>` — you posted a plan on issue `<N>`, awaiting review.
  - `<repo>-pr-<P>` — you opened/updated PR `<P>`.
  - `project-issue-<N>` — you opened a NEW issue, or materially updated an existing one (especially cross-repo), that the orchestrator needs to triage/route/own. Use whenever you surface work to the tracker — the orchestrator keeps triage, scope, routing, and close authority; the trigger just hands it the pointer.

  Examples: plan on issue 667 → `cli-plan-667.trigger`; opened PR 61 → `cli-pr-61.trigger`; filed a cross-repo bug as issue 700 → `project-issue-700.trigger`. The orchestrator consumes the trigger after it acts. (A substantive push also reaches the orchestrator automatically; the trigger is the explicit "come look at this" pointer.)

**Don't message the orchestrator out-of-band.** To surface a finding or a newly-filed issue, file/comment the GH issue and drop `project-issue-<N>.trigger` — that's the only channel. Your message reaches the orchestrator, and it signals back with a trigger when there's follow-up for you.

**Cost model — why the watcher is cheap, and your part in keeping it cheap:**
- The watcher is a tiny background shell loop — it does **not** wake you or burn context on its own. You spend tokens only when it emits a real `work:` line and starts a turn; periodic silence is free.
- There are no "idle cycles" to keep cheap — when the watcher is quiet, do nothing at all (no issue reads, no file reads, no `gh` calls, no output).
- **Never read an issue or file "just to check" for work.** The trigger IS the check. Reading to discover "nothing changed" is the exact waste this design removes.

## Push Protocol — MANDATORY

**Every push of substantive commits (`feat:`, `fix:`, `refactor:`) MUST include an `.orchestrator-msg` file.** The pre-push hook **REJECTS** such pushes if it's missing. Bare notifications cause stale or wrong orchestrator state — this is blocked at the hook level, not just convention.

**Steps:**
1. Write `.orchestrator-msg` in the repo root (contents below).
2. Do NOT `git add` or commit it — it's a working-tree file the hook reads during push.
3. **Hand Ben the `git push` command — Ben pushes, not you.** The hook delivers your message to the orchestrator automatically, then cleans it up.
4. **If you forget:** the hook rejects with a clear message. Write the file, hand Ben the push again (no empty commit needed — the hook re-fires).

**What to include:** issues closed/progressed with commit hashes; test results (pass count, any skipped); cross-repo implications; plan-review requests; **if a fix was discovered during live testing, say so explicitly** (that's the context lost without a message); the current state of the work (committed vs published vs verified end-to-end).

**Which commits trigger the block:**
- BLOCKED: subjects matching `^(feat|fix|refactor)(\(scope\))?!?:` (substantive Conventional Commits, incl. breaking `!:`).
- ALLOWED: `chore:`/`docs:`/`style:`/`test:`/`ci:`/`build:`/`perf:` push silently.

**Emergency override (logged for audit):** `SKIP_ORCHESTRATOR_MSG=1 git push` — recorded in `decisions.log` with timestamp + commits. Use only when writing context is genuinely impossible right then.

## Plan Mode Gate — MANDATORY

**Every plan requires orchestrator approval. No exceptions.** When you enter plan mode for ANY feature — regardless of scope:
1. Write the plan.
2. **Post it as a comment on the GH issue the orchestrator's trigger named** (the issue is the durable plan record). For cross-repo work, post on the umbrella issue. `gh issue comment <N> --repo skrptiq/skrptiq-issues --body "$(cat <<'EOF' ... EOF)"`.
3. **STOP. Do not implement, do not start coding, do not approve your own plan.** Drop `touch /Users/bencrocker/Developer/skrptiq-orchestrator/triggers/<repo>-plan-<N>.trigger`.
4. Wait for the orchestrator's approval comment on the same issue before proceeding.

**Why the issue, not a scratch file:** it's the canonical record — immutable and searchable via `gh issue view`; ad-hoc files drift. **Plan and review live on the issue.** Only the orchestrator approves plans; implementing before approval risks rejection. Applies to everything that enters plan mode: features, engine boundary changes, storage changes.

## Communication Protocol
- **British English** throughout all code, docs, and UI copy.
- **No platitudes.** Direct answers only.
- **Challenge Ben.** He is not a developer. Push back on bad ideas, explain trade-offs honestly.

## GitHub Issues
Tracker: `skrptiq/skrptiq-issues`. Open issues for this repo: `gh issue list -R skrptiq/skrptiq-issues --label "cli" --state open`.
- Single-repo issue → comment with the commit hash, close it.
- Your part of a cross-repo issue → comment, remove the `cli` label, do NOT close.
- Found a bug in another repo → open a GH issue with appropriate labels, then drop `project-issue-<N>.trigger`.

Labels: `app`, `hub`, `internal`, `web`, `cli`, `cross-repo`, `content`, `schema`, `bug`, `enhancement`.

## Context Budget — IMPORTANT
Your context window is expensive. Protect it.

**At session start, read ONLY:** this `CLAUDE.md`; the GH issue the trigger names (its latest orchestrator-directive comment); earlier comments on that issue, only if the directive needs them; files the directive explicitly references.

**Do NOT read unless the current task requires it:** `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `docs/HISTORY.md` (session-end only). Cross-repo files — never; ask the orchestrator if you need cross-repo context.

**Cross-repo knowledge:** the orchestrator maintains `../skrptiq-orchestrator/knowledge/`. `rg 'summary:' ../skrptiq-orchestrator/knowledge/K-*.md` to scan; `rg -l 'repos:.*cli' ../skrptiq-orchestrator/knowledge/` for entries affecting this repo; read the file only if the summary is relevant.

**Rule: if the issue directive doesn't mention it, don't load it.**

---

## Tech Stack
- **Language:** Go
- **Terminal input:** readline (github.com/chzyer/readline) — inline REPL with terminal scrollback
- **Styling:** lipgloss (Charm ecosystem) — output formatting only
- **Execution engine:** Imported from `skrptiq-app/engine/` as Go module
- **Storage:** `~/Library/Application Support/skrptiq/data/skrptiq.db` (WAL-mode SQLite, shared with desktop app via engine storage package)
- **Distribution:** single binary — brew, GitHub releases, go install

## Architecture

### What the CLI owns
- Interactive REPL with readline input and fmt.Println output
- Terminal scrollback — all output persists in the terminal's native scroll buffer
- Workspace detection (current directory context)
- Pipe support (stdin/stdout for composability)
- Natural language input → workflow selection
- Session context management

### What the engine owns (shared with desktop app)
- Execution engine (workflows, loops, gate steps, step types)
- Profile system (voice profiles, persona dials, audience profiles)
- MCP client (connect to servers, discover tools, call tools)
- Workflow runner (plan builder, variable resolution, step execution)
- Hub client (import skrpts, check for updates)
- Storage (single SQLite DB, all CRUD operations)

### Boundary rule
Import the engine module only. Never import app-internal Electron packages. If you need something from the app that isn't in the engine, flag it to the orchestrator — it probably needs extracting into the engine.

### Integration contract
If you ever build import, file write, or DB write functionality: read `../skrptiq-orchestrator/docs/SKRPT-INTEGRATION-CONTRACT.md` first. It defines the exact data shape the engine expects. The CLI currently reads from the shared DB (populated by the app) — any future write paths must follow the same contract.

## Code Conventions
- British English in all user-facing strings, comments, and docs
- Prefer simplicity over cleverness — no premature abstraction
- Own-repo commits only — never modify sibling repos
- Standard Go project layout
- `go fmt` and `go vet` before every commit
- **No silent failures on critical paths** — if engine or DB fails to load, exit immediately with actionable stderr. Never continue with a broken subsystem.

## Architectural Invariants — MANDATORY

Rules that hold across every session. Violations are bugs, not trade-offs. Directives will not repeat these — they live here.

**Module boundary:**
- **Import the `engine` module only.** Never import `electron-app`-internal packages. The CLI is a separate consumer of the engine; the engine is the only contract.
- **Single binary distribution.** No native add-ons, no companion processes. One Go binary.
- **Fail fast on startup-critical errors.** If the engine can't initialise, DB can't open, or required config is missing, exit non-zero with actionable stderr.

**Auto-release chain (GH#539):**
- **CLI releases auto-fire on App releases.** No manual `git tag` for engine-driven CLI patches.
- **Caller-release.yml fetches latest** — no version pin in Hub's CI.
- **Trigger to act manually:** dispatch workflow fails OR a new CLI tag doesn't appear after an App release. Otherwise stand watch.

**Process:**
- **Plan Mode Gate** — every non-trivial plan goes to orchestrator review before code. See Plan Mode Gate above.

## Failure Classification

| Class | CLI behaviour |
|-------|-------------|
| **Startup-critical** (engine, DB) | Exit with error message to stderr. Non-zero exit code. |
| **Per-operation** (MCP call, API fetch) | Print error inline, continue session. |

The CLI must never enter the interactive session with a broken engine or DB. Fail fast, fail loud.

## Testing — MANDATORY
- `go test ./...` — required for ALL changes. Do not commit without tests passing.
- **Write tests alongside code.** Every new handler, function, or behaviour change must include tests that validate it works and will catch regressions.
- **Test categories:**
  - **Unit tests** — pure functions, type methods, parsing logic, command routing
  - **Integration tests** — engine queries against a test database, command handlers with real data
  - **No mocks unless unavoidable** — prefer a real test database over mocking storage
- **Test file placement:** `*_test.go` next to the code it tests
- Report results in your `.orchestrator-msg` push summary.
