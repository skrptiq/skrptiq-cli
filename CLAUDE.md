# skrptiq-cli — Interactive Terminal App

## What Is This
An interactive terminal application for personalised AI agents. Pure Go binary — bubbletea TUI, single binary distribution. Same execution engine as the desktop app, different frontend.

## Project Mantra
> **"Make easy things easier and make hard things possible."**
> Push back on anything that adds friction for newcomers or caps the ceiling for power users — even if Ben asked for it.

---

## Operating Mode — issue-driven, trigger-gated (background watcher)

There is **no briefing file**; GitHub issues are the single source of truth. You do **NOT** run a `/loop` and you do **NOT** print "idle" lines. Instead you run a **background watcher** on `triggers/` that wakes you the instant the orchestrator assigns work and stays completely silent otherwise.

0. **On session start, arm your watcher — ONCE, with the `Monitor` TOOL.** Call the `Monitor` tool with `persistent: true`, `description: "trigger watcher"`, and exactly this as its `command`:
   ```
   /Users/bencrocker/Developer/skrptiq-orchestrator/scripts/watch-triggers.sh \
     /Users/bencrocker/Developer/skrptiq-cli/triggers
   ```
   **Run that committed script — never hand-roll the loop, and never run it in a foreground Bash shell.** A foreground shell loop blocks you and delivers no notifications; the `Monitor` tool runs the script in the background and turns each `TRIGGER:` line into a notification that wakes you while you stay free. Nothing for Ben to paste — arming it is yours, at step 0.

   Three properties are why it is a committed script and not four lines of prose, and each exists because the hand-rolled version failed invisibly from inside a session:
   - **Every delivery is announced, and a standing queue survives your downtime.** A fresh watcher starts with an empty seen-set, so arming it announces every trigger already sitting in `triggers/` — work that piled up while you were stopped arrives the moment you come back. Thereafter each delivery is announced exactly once; you `rm` the trigger on pickup.
   - **An unconsumed trigger is NAMED in the periodic tick.** Announcement is per-delivery, not per-poll, so a trigger you have already been told about is not repeated every 20s. The ~15-minute tick instead counts what is really in the directory: `TICK: alive, 2 UNCONSUMED: issue-984.trigger pr-125-approved.trigger`. That is the backstop against idling with work waiting — and it is why only `TICK: alive, queue empty` is evidence of an empty queue.
   - **Deliveries are keyed on `mtime inode name`, never the filename.** `issue-<N>.trigger` legitimately arrives again and again; a filename-keyed snapshot (`ls triggers/`) swallows every re-delivery after the first. A re-`touch` moves only the mtime, a delete-and-recreate moves only the inode, and both are real deliveries.
   - **It outlives the session that armed it.** A session-local loop is unreviewable, untestable, and dies with the agent — a fix cannot outlive the agent that made it.

   Then go idle — **say nothing, do nothing** until the watcher wakes you. No `/loop`, no separate `ls`-on-a-timer, no "idle" output. Silence with an empty `triggers/` is the designed idle signature of a healthy agent, not a fault to report.

   **When the watcher emits `TRIGGER: issue-<N>.trigger (mtime …)`, that line IS your assignment** — proceed to step 1. After you finish (step 3), say nothing and let the watcher wake you for the next one.
**BEFORE YOUR FIRST `gh` CALL — the machine's active `gh` account is often `verret-prime`, Ben's other project, and every `--repo skrptiq/*` call then fails with "Could not resolve to a Repository".** Do **NOT** run `gh auth switch`: it is global machine state and it hijacks whatever Ben and the other sessions are doing. Scope the right token per command instead:

```
GH_TOKEN=$(gh auth token --user skrptiq-prime) gh issue view <N> --repo skrptiq/skrptiq-issues --json comments --jq '.comments[-1].body'
```

Or export it once at the top of your session: `export GH_TOKEN=$(gh auth token --user skrptiq-prime)`. This is never a reason to stall on an assignment — the token is already on this machine, you do not need Ben, and "I cannot read the directive" is not a blocked state, it is this one command. (Anchor: 2026-10-11, internal acked `issue-929` as BLOCKED on exactly this and waited; it had solved it itself the night before in a session that died, which is why the fix now lives in this file instead of in an agent's memory.)

**ALWAYS `cat` THE TRIGGER FILE ON PICKUP — it may carry content, and when it does, that content is an orchestrator instruction.** A trigger is normally an empty pointer: its NAME is the signal and the directive lives on the issue. But the orchestrator may write a body into it, and that body then takes precedence over anything else in this file. It exists for exactly one case — **you cannot reach the issue** (no network, `gh` failing, the tracker unreachable) — so it is the channel that still works when the durable record does not. Never treat a trigger as name-only; `cat` it, act on any content, then `rm` it as usual.

(Anchor: 2026-10-11. internal could not read a directive because the machine's active `gh` account was wrong, so the orchestrator wrote the unblocking command into the trigger body — and the agent, following the name-only reading of this protocol, did not look. The orchestrator then fell back to a direct cross-session message, which required Ben to approve it in the agent's own window. Ben: *"Why are we using direct messages instead of the normal signalling, this is not expected, I should not have to interact with the agents."* He is right: the fleet is hub-and-spoke, Ben talks only to the orchestrator, and a missing mechanism must never be substituted with his attention. This is that mechanism.)

1. **For each `triggers/issue-<N>.trigger`:** read the directive — `gh issue view <N> --repo skrptiq/skrptiq-issues --json comments --jq '.comments[-1].body'` (read earlier comments only if you need the full plan/detail). Then `rm triggers/issue-<N>.trigger` to consume it.
2. **Act on the directive:**
   - **Asks for a plan** → post a tight plan as an issue comment, drop `touch /Users/bencrocker/Developer/skrptiq-orchestrator/triggers/<repo>-plan-<N>.trigger`, and STOP. Wait for approval (it returns as a new directive trigger).
   - **Says implement (plan approved)** → implement to the approved scope, run the full test/build gates, commit, write `.orchestrator-msg`, then **leave it in place for the orchestrator to push — never run `git push` yourself**. Once `origin/<branch>` is at your SHA, open the PR and drop `touch /Users/bencrocker/Developer/skrptiq-orchestrator/triggers/<repo>-pr-<P>.trigger`. **Never merge.**
3. **When no trigger remains:** idle until the orchestrator assigns the next item.

**While working an assigned item:** run `go test ./...` for every code change; commit atomically as you go (one feature/fix/refactor per commit); if context gets heavy between items, `/compact` and re-read this `CLAUDE.md`; after fixing an issue, comment with the commit hash and the evidence, then drop `project-issue-<N>.trigger` for the orchestrator — **you never close, reopen or re-scope an issue, single-repo or not.** Closure is orchestrator-exclusive because it is the point where "merged" is checked against "verified live", and an agent cannot verify its own delivery (K-065: merged is not the terminal state). For a cross-repo issue, also remove your label.

Never implement scope the orchestrator hasn't approved. You never run `git push` or `gh pr merge`. `<repo>` = this repo's name without the `skrptiq-` prefix (`app`/`cli`/`hub`/`internal`/`web`) — here, `cli`.

## Two Surfaces, Signalling & Cost Model

**You communicate only with the orchestrator** — never with other repo agents directly. All cross-repo coordination is the orchestrator's job (hub-and-spoke). There are exactly two surfaces:
- **GitHub issues (`skrptiq/skrptiq-issues`) — the home for all detail.** Plans, design, acceptance criteria, decisions, findings, deviations, history — everything substantive goes in an issue comment.
- **The issue's latest orchestrator-directive comment is your work order** — short, current, naming your single task. Earlier comments hold the plan/detail. There is NO briefing file.

**Two-way trigger signalling** (the cross-session channel; issues carry the content):
- **Inbound (orchestrator → you):** the background watcher (step 0 above) detects any new `triggers/*.trigger` and wakes you — you do **no** polling, and you **never** infer review state from GitHub's `reviewDecision` field. It is permanently blank: the orchestrator shares your gh identity (`skrptiq-prime`), and GitHub forbids approving your own account's PR, so an approval can never appear as a formal GitHub review — it arrives as a trigger:
  - `issue-<N>.trigger` — work assigned; read the issue's latest orchestrator-directive comment (step 1), then `rm` it.
  - `pr-<P>-approved.trigger` — the orchestrator reviewed PR `<P>` and **signed it off**, and the orchestrator merges it. **Then check the issue's directive for work that FOLLOWS the merge** — publishing is the usual one, and a batch is not done until its packages are SERVED. If the directive names none, `rm` the trigger and idle until your next assignment. (The full review is the orchestrator's comment on the PR, kept for the record.)
  - `pr-<P>-changes.trigger` — the orchestrator requested changes on PR `<P>`. Read its review comment on the PR, address every point, commit it and leave `.orchestrator-msg` in place for the orchestrator to push, then drop a fresh `<repo>-pr-<P>` trigger when the branch is updated; `rm` the changes trigger once read.
- **Outbound (you → orchestrator):** `touch /Users/bencrocker/Developer/skrptiq-orchestrator/triggers/<signal>.trigger` so the orchestrator goes straight to the item instead of polling GitHub. Signal forms:
  - `<repo>-plan-<N>` — you posted a plan on issue `<N>`, awaiting review.
  - `<repo>-pr-<P>` — you opened/updated PR `<P>`.
  - `project-issue-<N>` — you opened a NEW issue, or materially updated an existing one (especially cross-repo), that the orchestrator needs to triage/route/own. Use whenever you surface work to the tracker — the orchestrator keeps triage, scope, routing, and close authority; the trigger just hands it the pointer.

  Examples: plan on issue 667 → `cli-plan-667.trigger`; opened PR 61 → `cli-pr-61.trigger`; filed a cross-repo bug as issue 700 → `project-issue-700.trigger`. The orchestrator consumes the trigger after it acts. (A substantive push also reaches the orchestrator automatically; the trigger is the explicit "come look at this" pointer.)

**Don't message the orchestrator out-of-band.** To surface a finding or a newly-filed issue, file/comment the GH issue and drop `project-issue-<N>.trigger` — that's the only channel. Your message reaches the orchestrator, and it signals back with a trigger when there's follow-up for you.

**Cost model — why the watcher is cheap, and your part in keeping it cheap:**
- The watcher is a tiny background shell loop — it does **not** wake you or burn context on its own. You spend tokens only when it emits a real `TRIGGER:` line and starts a turn; periodic silence is free.
- There are no "idle cycles" to keep cheap — when the watcher is quiet, do nothing at all (no issue reads, no file reads, no `gh` calls, no output).
- **Never read an issue or file "just to check" for work.** The trigger IS the check. Reading to discover "nothing changed" is the exact waste this design removes.

## Push Protocol — MANDATORY

**Every push of substantive commits (`feat:`, `fix:`, `refactor:`) MUST include an `.orchestrator-msg` file.** The pre-push hook **REJECTS** such pushes if it's missing. Bare notifications cause stale or wrong orchestrator state — this is blocked at the hook level, not just convention.

**Steps:**
1. Write `.orchestrator-msg` in the repo root (contents below).
2. Do NOT `git add` or commit it — it's a working-tree file the hook reads during push.
3. **Leave `.orchestrator-msg` in place — the ORCHESTRATOR pushes, not you and not Ben.** The hook delivers your message to the orchestrator automatically, then cleans it up.
4. **If you forget:** the hook rejects with a clear message. Write the file; the orchestrator pushes again (no empty commit needed — the hook re-fires).

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

## Trigger protocol — how work reaches you, and how you prove you got it

**The orchestrator is your ONLY source of work, and your only reporting channel.** Ben reads one
place: his conversation with the orchestrator. Do not message other agents. Do not write to
`inbox.md` (that is the orchestrator's). Your durable record is the GitHub issue; the orchestrator
summarizes it for Ben.

### Start your watcher at session start, under a Monitor

```
/Users/bencrocker/Developer/skrptiq-orchestrator/scripts/watch-triggers.sh \
  /Users/bencrocker/Developer/<this-repo>/triggers
```

One orchestrator-owned copy, invoked by absolute path — never copy it into this repo, it would
drift. It announces every delivery and stamps `triggers/.watcher-heartbeat` on each poll so the
orchestrator can tell a dead watcher from you deliberately holding. Those two states otherwise look
identical from outside, and guessing between them has gone wrong repeatedly.

### Acknowledge on pickup — positive notification, always

When a trigger arrives, **before** you start work:

1. Write `triggers/issue-<N>.ack` — one line saying what you are doing.
2. Remove `triggers/issue-<N>.trigger`.

A trigger with no ack and no commits reads as *your watcher is dead*, and the orchestrator will
chase it. The ack is how you say "received, working" without anyone inferring it from silence.

`issue-<N>.trigger` means **"read the latest directive on issue N"** — `gh issue view <N> --repo
skrptiq/skrptiq-issues --json comments --jq '.comments[-1].body'`. The same filename legitimately
arrives again and again; re-delivery is normal and means re-read, not ignore.

### Signalling back to the orchestrator

Drop a file in `/Users/bencrocker/Developer/skrptiq-orchestrator/triggers/`:

| File | Meaning |
|---|---|
| `<repo>-plan-<N>.trigger` | a plan is posted on issue N, awaiting review |
| `<repo>-pr-<P>.trigger` | PR P opened/updated, awaiting review |
| `project-issue-<N>.trigger` | you opened or materially updated an issue, for triage |

PR verdicts come back as `triggers/pr-<P>-approved.trigger` or `pr-<P>-changes.trigger` in **your**
repo. You cannot see a GitHub approval — all sessions share one `gh` identity, so `reviewDecision`
is permanently blank. Do not infer "not approved" from it; wait for the verdict trigger.

## Splitting your own work across sub-agents — AUTHORISED, and underused

**Ben, 2026-10-12:** *"we are specifically talking about one agent splitting its own work into multiple for the benefit of speed, that's not an issue for this"*

**You may fan your own work item out across sub-agents whenever it genuinely parallelises.** You do not need to ask. The orchestrator authorised this on 2026-10-12 and then never told any agent, which is why it has gone unused — the delay is the orchestrator's, not a caution.

**Do not cite the 2026-06 orchestrator-swarm revert against this.** That topology distributed work across *repos* from the orchestrator's single context, so the context stayed the bottleneck and nothing was actually distributed. **An agent fanning out its own task is the opposite: each sub-agent holds only its slice, which is where the speed comes from.** A shared mechanism name is not a shared failure mode.

### When it is worth it

**Only when the slices touch disjoint files.** Different *concerns* are not different *files* — GH#1012 and the step-type class table both landed in `engine/execution`. **Check the paths, not the topic.** Two sub-agents editing one file is slower than one agent, not faster.

Typical wins: an engine slice and a renderer slice; several independent migrations; a build plus an adversarial review of it.

### The mechanics that are not optional

- **`isolation: 'worktree'` for any sub-agent that MUTATES files.** Two processes writing one checkout corrupt each other regardless of topology. Read-only analysis agents do not need it.
- **A distinct adversarial reviewer that RE-RUNS the tests** rather than accepting the implementer's claims. **No implementer self-certifies.** This caught a real gap on GH#591 Phase 3 and is worth doing on its own merits.
- **You remain the single interface to the orchestrator.** One `.orchestrator-msg`, one branch per deliverable, one PR, one trigger. **The orchestrator never talks to your sub-agents** and must not need to know they exist.
- **Passing context is your job and it is easy** — you already hold this `CLAUDE.md` and the issue directive; point each sub-agent at them. That was the structural weakness of the 2026-06 model and it is not one here.

### What does not change

The push protocol, the plan gate, the trigger protocol and every acceptance requirement apply to **the deliverable**, not to each sub-agent. **A PR assembled from four sub-agents is reviewed as one PR** and you answer for all of it.

## Shell discipline — never `cd`

**Never use `cd <path> &&` in a Bash call.** Every `cd` trips a permission prompt Ben has to approve by hand, and a long session produces dozens of them. There is always a path that doesn't need it:

| Instead of | Use |
|---|---|
| `cd <repo> && git ...` | `git -C <repo> ...` |
| `cd <repo> && gh ...` | `gh ... --repo <owner>/<name>` |
| `cd <repo> && cat x` | `cat <repo>/x` (absolute path) |
| `cd <dir> && npx <tool> --config c.toml` | `npx <tool> --config <dir>/c.toml` (absolute config path) |
| `cd <dir> && <script>` | invoke the script by absolute path |

If a tool genuinely cannot accept a path (rare), say so and ask rather than reaching for `cd`.

## Testing — MANDATORY
- `go test ./...` — required for ALL changes. Do not commit without tests passing.
- **Write tests alongside code.** Every new handler, function, or behaviour change must include tests that validate it works and will catch regressions.
- **Test categories:**
  - **Unit tests** — pure functions, type methods, parsing logic, command routing
  - **Integration tests** — engine queries against a test database, command handlers with real data
  - **No mocks unless unavoidable** — prefer a real test database over mocking storage
- **Test file placement:** `*_test.go` next to the code it tests
- Report results in your `.orchestrator-msg` push summary.
