#!/usr/bin/env bash
#
# Does this repo compile against the engine revision `.engine-pin` names?
#
# GH#1118 — ONE QUESTION, ONE SCRIPT, TWO CALLERS.
#
# This is check 3 of `engine-chain-check.sh`, lifted out so `ci.yml` can ask it
# per-PR without asking the other two. It is not a copy: the watchdog now calls
# this file, so there is a single implementation of the rule (GH#1058 — one
# authority per rule; two copies of a compile check drift, and the drift is
# invisible until they disagree).
#
# WHY CI MUST NOT JUST RUN `engine-chain-check.sh`: that script's checks 1 and 2
# ask whether app's release chain is DELIVERING, and they query the releases API
# before check 3 is ever reached. Running them per-PR would fail cli's PRs
# whenever app simply had not released recently — importing the exact coupling
# this split exists to confine to one check. This script touches no network and
# asks nothing about releases.
#
# WHAT IT IS FOR, stated plainly because the absence of it caused a 12-day
# outage: `ci.yml` builds against app `main`, and the RELEASE builds against the
# pin. Those are different engines. CI's existing "Pin sanity" step asserts the
# pin is 40-hex and an ancestor of main — it asserts the pin EXISTS and never
# that it WORKS. `18c63f01` was a valid ancestor of main for twelve days while
# cli HEAD could not compile against it at all, so pin-sanity passed on every PR
# and nothing went red until a SCHEDULED watchdog run, long after the merge.
#
# Verified on real history rather than a fixture (GH#1118):
#
#   cli 48ed3af^  vs pin 18c63f01  -> PASS   the commit before the floor rise
#   cli 48ed3af   vs pin 18c63f01  -> FAIL   GH#1221, which imported engine/outputs
#   cli HEAD      vs pin b13bc366  -> PASS   today
#
# So it goes red on the commit that actually caused the incident, not on one
# built to make it go red.
#
# IT CANNOT BE REDDENED BY A SIBLING'S MERGE. The pin is a frozen SHA, so this
# only fails when THIS repo changes. That is what makes it safe to make blocking
# — a required check another repo can turn red is a required check that gets
# switched off, which is why `engine-main-drift.yml` (subject: app `main`) stays
# scheduled and classifying instead.
#
# Usage:
#   APP_CHECKOUT=/path/to/skrptiq-app scripts/engine-pin-build.sh
#
# Env:
#   APP_CHECKOUT  a checkout of skrptiq-app containing the pinned revision
#   PIN_FILE      default <repo root>/.engine-pin
#
# Exit: 0 compiles · 1 does not compile · 2 could not establish (fail closed)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PIN_FILE="${PIN_FILE:-$REPO_ROOT/.engine-pin}"
APP_CHECKOUT="${APP_CHECKOUT:-}"

err()  { echo "::error::$*" >&2; }
note() { echo "$*"; }

fatal() {
  err "engine-pin-build could not establish whether the pin builds: $*"
  err "Treating an unverifiable pin as a broken pin (fail closed) — do not read this as 'the pin is fine'."
  exit 2
}

[ -n "$APP_CHECKOUT" ] || fatal "APP_CHECKOUT is unset — there is no app checkout to take the engine from"
[ -d "$APP_CHECKOUT/.git" ] || fatal "APP_CHECKOUT=$APP_CHECKOUT is not a git checkout"
[ -f "$PIN_FILE" ] || fatal "$PIN_FILE is missing — there is no pin to build against"
command -v go >/dev/null 2>&1 || fatal "go is not on PATH — an unverified pin is not a working pin"

PIN="$(tr -d '[:space:]' < "$PIN_FILE")"
[[ "$PIN" =~ ^[0-9a-f]{40}$ ]] || fatal ".engine-pin is not a 40-hex commit SHA (got '${PIN}')"

# Reproduce the RELEASE build's layout rather than approximating it: release.yml
# checks the engine out AT THE PIN next to the CLI, and go.mod's
# `replace github.com/skrptiq/engine => ../skrptiq-app/engine` resolves to it.
#
# `git archive` keeps this read-only on the app checkout — no worktree, no branch
# switch, nothing a concurrently-working sibling repo would notice. That matters:
# app's working tree is a THIRD engine subject (whatever branch and dirty state it
# happens to be in), and grading against it would be grading something nobody has
# released or merged.
#
# HEAD, not the working tree: the question is whether what we would release
# builds, not whether an in-progress edit does.
PINTMP="$(mktemp -d)"
trap 'rm -rf "$PINTMP"' EXIT
mkdir -p "$PINTMP/skrptiq-app" "$PINTMP/skrptiq-cli"

git -C "$APP_CHECKOUT" archive "$PIN" engine 2>/dev/null | tar -x -C "$PINTMP/skrptiq-app" \
  || fatal "could not extract engine/ at ${PIN:0:7} from the app checkout"
[ -f "$PINTMP/skrptiq-app/engine/go.mod" ] \
  || fatal "engine/go.mod absent at ${PIN:0:7} — the pin does not name a revision with an engine"
git -C "$REPO_ROOT" archive HEAD 2>/dev/null | tar -x -C "$PINTMP/skrptiq-cli" \
  || fatal "could not archive this repo at HEAD"

if BUILD_OUT="$(go build -C "$PINTMP/skrptiq-cli" ./... 2>&1)"; then
  note "PIN BUILD ok    — HEAD compiles against the pinned engine ${PIN:0:7}."
  exit 0
fi

err "THE PINNED ENGINE DOES NOT BUILD: this repo at HEAD does not compile against ${PIN:0:7}, the revision .engine-pin names."
err "release.yml checks the engine out at that pin, so EVERY release build would fail at the Build step — there would be no releasable commit on this branch."
err "ci.yml's other build uses app main, which has the symbols the pin does not, so it would stay green while the release path is dead. That is GH#1118 and it is why this check exists."
err "If this failed because you imported an engine symbol that is not released yet: that is the finding, not a false positive. A consumer resolves releases, not a sibling's main (GH#1234). The fix is an app release, NOT a hand-bump of .engine-pin."
while IFS= read -r line; do [ -n "$line" ] && err "  $line"; done <<< "$BUILD_OUT"
exit 1
