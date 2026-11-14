#!/usr/bin/env bash
#
# engine-chain-check.sh — assert that the App -> CLI auto-release chain
# (GH#539) is actually delivering, and FAIL LOUDLY when it is not.
#
# WHY THIS EXISTS (GH#1079).
#
# The chain never fired once. `dispatch-cli-after-release.yml` in the App was
# switched to `on: release: published` by GH#564 on 2026-05-17 (that workflow
# is gone -- GH#1086 inlined the dispatch into `build-macos.yml` -- so do not
# go looking for it; this paragraph is dated history, not current layout); the App's
# releases are created by `softprops/action-gh-release` under the default
# GITHUB_TOKEN, and GitHub does not raise workflow-triggering events for
# actions taken with GITHUB_TOKEN. So the sender stopped starting, in silence,
# across eight App releases and three months -- and the only symptom was a CLI
# tag that never appeared. Nothing reports a dispatch that was never sent:
# zero runs is indistinguishable from a quiet period.
#
# That is the defect this script addresses. It does not replace the chain and
# it cannot repair it -- the cause lives in the App repo. It makes the chain's
# ABSENCE an alert instead of an inference, so the next outage is noticed in a
# day rather than discovered three months later by an unrelated investigation.
#
# THREE INDEPENDENT CHECKS. They fail for different reasons on purpose; any
# one alone would have missed a real case.
#
#   1. NO DISPATCH ARRIVED — for the App's newest release, did a
#      `repository_dispatch` run of OUR `release.yml` happen at or after that
#      release was published? This is the check that catches the actual GH#1079
#      fault, and it fires even when the release carried no engine change
#      (where check 2 is legitimately silent).
#
#      IT TESTS RECEIPT AT OUR END, NOT A FILENAME AT THEIRS (GH#1192). It used
#      to count runs of the App's `dispatch-cli-after-release.yml`, which made
#      the gate depend on a sibling repo's file layout: when GH#1086 inlined the
#      dispatch into `build-macos.yml` and deleted that workflow, BOTH ways of
#      implementing the repair left this check red forever -- delete the file and
#      the lookup goes fatal, keep it and the count is zero. Receipt is the
#      property we actually care about, it is satisfied by either sender, and it
#      is measured from our own run history rather than inherited from theirs.
#
#      The window is filtered to `repository_dispatch` SERVER-SIDE rather than
#      in jq. `--limit 100` of all events would narrow as this repo got busier
#      -- 23 `push` runs against 7 dispatches in the same window when this was
#      written -- and pushes are the kind that accumulate. A window that
#      degrades with activity is a gate with a shelf life.
#
#   2. SHIPPED ENGINE IS BEHIND — does `engine/` differ between our .engine-pin
#      and the App's newest release tag? This catches delivery failing anywhere
#      downstream of the dispatch (receiver, tag job, build) and is the check
#      that speaks to what we actually ship.
#
#   3. THE PIN DOES NOT BUILD — does this repo still compile against the engine
#      revision `.engine-pin` names? Added for GH#1084. `release.yml` checks out
#      the engine AT THE PIN, so if HEAD has outgrown it every release build
#      fails -- and nothing says so until someone cuts a tag. ci.yml builds
#      against app `main`, never against the pin, so this is invisible to it.
#      It has been true since 2026-07-23 and was found only because the dead
#      chain meant no tag was ever pushed to discover it.
#
# Check 2 reuses the chain's OWN rule for "the engine changed" -- a diff of
# `engine/` between two app commits, exactly as the sender's gate computes it.
# Re-deriving that condition differently here would make this a second
# authority whose only possible contribution is disagreement (the GH#1058
# lesson), and it would disagree on precisely the releases that matter.
#
# FAIL CLOSED. A lookup that errors or returns nothing exits non-zero. A
# watchdog that reports "fine" when it could not see is the disease it was
# written to cure.
#
# Usage:
#   GH_TOKEN=$(gh auth token --user skrptiq-prime) \
#     APP_CHECKOUT=/path/to/skrptiq-app scripts/engine-chain-check.sh
#
# Env:
#   APP_REPO      default skrptiq/skrptiq-app
#   SELF_REPO     default skrptiq/skrptiq-cli -- whose release.yml receipts
#                 check 1 counts
#   PIN_FILE      default <repo root>/.engine-pin
#   APP_CHECKOUT  a full checkout of APP_REPO (tags + history) for check 2
#   GRACE_HOURS   default 6 -- a release younger than this is still in flight,
#                 so a missing dispatch is reported but not failed

set -uo pipefail

APP_REPO="${APP_REPO:-skrptiq/skrptiq-app}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PIN_FILE="${PIN_FILE:-$REPO_ROOT/.engine-pin}"
GRACE_HOURS="${GRACE_HOURS:-6}"
SELF_REPO="${SELF_REPO:-skrptiq/skrptiq-cli}"
RELEASE_WF="release.yml"

# In GitHub Actions `::error::` turns a line into an annotation; locally it is
# just a prefix. Either way the text is the same, so the terminal run and the
# CI run say the same thing.
err()  { echo "::error::$*" >&2; }
note() { echo "$*"; }

fatal_lookup() {
  err "engine-chain-check could not establish the chain's state: $*"
  err "Treating an unverifiable chain as broken (fail closed) — do not read this as 'the chain is fine'."
  exit 2
}

command -v gh >/dev/null 2>&1 || fatal_lookup "gh is not on PATH"

# --- the App's newest published release -------------------------------------
#
# NOT `/releases/latest`: that endpoint excludes prereleases, and EVERY App
# release is a prerelease (v0.1.0-beta.NN). It would return a years-old
# release or 404, and the watchdog would quietly compare against the wrong
# thing. Sort the full list by publication instead.
LATEST="$(gh api "repos/${APP_REPO}/releases" --paginate \
  --jq '[.[] | select(.draft == false)] | sort_by(.published_at) | last | [.tag_name, .published_at] | @tsv' 2>/dev/null)"
[ -n "$LATEST" ] || fatal_lookup "no published release found in ${APP_REPO}"

REL_TAG="$(printf '%s' "$LATEST" | cut -f1)"
REL_PUB="$(printf '%s' "$LATEST" | cut -f2)"
[ -n "$REL_TAG" ] && [ -n "$REL_PUB" ] || fatal_lookup "could not parse the newest release of ${APP_REPO}"

# Portable epoch parse: GNU date on the runner, BSD date when Ben runs it on macOS.
to_epoch() {
  date -u -d "$1" +%s 2>/dev/null || date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$1" +%s 2>/dev/null
}
REL_EPOCH="$(to_epoch "$REL_PUB")"
[ -n "$REL_EPOCH" ] || fatal_lookup "could not parse release timestamp '${REL_PUB}'"
AGE_HOURS=$(( ( $(date -u +%s) - REL_EPOCH ) / 3600 ))

note "App newest release : ${REL_TAG}  (published ${REL_PUB}, ${AGE_HOURS}h ago)"
note "CLI engine pin     : $(tr -d '[:space:]' < "$PIN_FILE" 2>/dev/null || echo '<unreadable>')"
note ""

IN_FLIGHT=0
[ "$AGE_HOURS" -lt "$GRACE_HOURS" ] && IN_FLIGHT=1
FAILED=0

# --- check 1: did a dispatch ARRIVE at all? ---------------------------------
RUNS="$(gh run list --repo "$SELF_REPO" --workflow "$RELEASE_WF" \
  --event repository_dispatch --limit 100 \
  --json createdAt --jq "[.[] | select(.createdAt >= \"${REL_PUB}\")] | length" 2>/dev/null)"
case "$RUNS" in
  ''|*[!0-9]*) fatal_lookup "could not list repository_dispatch runs of ${RELEASE_WF} in ${SELF_REPO}" ;;
esac

if [ "$RUNS" -gt 0 ]; then
  note "CHECK 1 ok      — ${RUNS} dispatch(es) reached ${SELF_REPO} since ${REL_TAG} was published."
elif [ "$IN_FLIGHT" -eq 1 ]; then
  note "CHECK 1 pending — no dispatch received yet, but ${REL_TAG} is only ${AGE_HOURS}h old (grace ${GRACE_HOURS}h)."
else
  FAILED=1
  err "NO DISPATCH ARRIVED: ${APP_REPO} published ${REL_TAG} ${AGE_HOURS}h ago and not one repository_dispatch has reached ${SELF_REPO}'s ${RELEASE_WF} since."
  err "Nothing told this repo to build, so no CLI release can exist for that engine. This tests RECEIPT at our end, so it holds whichever workflow the App sends from — it asserts nothing about a file in a sibling repo (GH#1192)."
  err "Known cause (GH#1079): a release created under the default GITHUB_TOKEN raises no workflow-triggering event, so a sender gated on 'on: release: published' never starts. Fix belongs in ${APP_REPO}."
fi

# --- check 2: is the engine we ship behind the engine they released? --------
if [ ! -f "$PIN_FILE" ]; then
  fatal_lookup "${PIN_FILE} is missing — there is no pin to compare"
fi
PIN="$(tr -d '[:space:]' < "$PIN_FILE")"
[[ "$PIN" =~ ^[0-9a-f]{40}$ ]] || fatal_lookup "${PIN_FILE} is not a 40-hex SHA (got '${PIN}')"

if [ -z "${APP_CHECKOUT:-}" ] || [ ! -d "${APP_CHECKOUT}/.git" ]; then
  fatal_lookup "APP_CHECKOUT is not a git checkout of ${APP_REPO} — check 2 cannot run, and a skipped check is not a passed check"
fi

git -C "$APP_CHECKOUT" cat-file -e "${PIN}^{commit}" 2>/dev/null \
  || fatal_lookup "pinned commit ${PIN} is not present in the ${APP_REPO} checkout"
git -C "$APP_CHECKOUT" rev-parse -q --verify "refs/tags/${REL_TAG}" >/dev/null 2>&1 \
  || fatal_lookup "release tag ${REL_TAG} is not present in the ${APP_REPO} checkout (fetch tags)"

if git -C "$APP_CHECKOUT" diff --quiet "${PIN}..${REL_TAG}" -- engine/; then
  note "CHECK 2 ok      — engine/ is identical between our pin and ${REL_TAG}; nothing was owed."
else
  CHANGED="$(git -C "$APP_CHECKOUT" diff --name-only "${PIN}..${REL_TAG}" -- engine/ | wc -l | tr -d ' ')"
  if [ "$IN_FLIGHT" -eq 1 ]; then
    note "CHECK 2 pending — ${CHANGED} engine/ file(s) ahead of our pin, but ${REL_TAG} is only ${AGE_HOURS}h old (grace ${GRACE_HOURS}h)."
  else
    FAILED=1
    # GH#1192 — PIN is read from $PIN_FILE in THIS checkout, so it is the pin
    # at HEAD, not the pin of whatever release is installed. Those are
    # different values (v0.0.21 shipped c4d7896; HEAD carried b12733c, written
    # four days later and never released), and this sentence used to claim the
    # HEAD pin was what every shipped binary was built against. Say HEAD,
    # because HEAD is this script's subject throughout: check 3 compiles
    # `git archive HEAD` against this same PIN. Pointing one sentence at the
    # release tag instead would put two subjects in one script and need a
    # second lookup to resolve — one authority per rule (GH#1058).
    #
    # The permissive claim itself is FOUNDED and must not be softened while the
    # subject is corrected: 29 of 95 catalogue packages pass the shipped
    # scanner and are refused by current main, with email-assistant refused by
    # both as the control.
    err "SHIPPED ENGINE IS STALE: ${CHANGED} file(s) under engine/ differ between our pin ${PIN:0:7} and the App's released ${REL_TAG}."
    err "A release built from this branch today would ship engine ${PIN:0:7}. A scanner on a stale engine grades against rules the shipped engine no longer holds — it passes content the current engine refuses (GH#1079), and it would fail permissive."
    err "Do not fix this by hand-bumping .engine-pin: that ships a current engine through a broken pipe and hides the outage. Repair the chain."
  fi
fi

# --- check 3: does HEAD still compile against the pinned engine? -----------
#
# Reproduces the release build's layout rather than approximating it:
# release.yml checks the engine out AT THE PIN next to the CLI, and go.mod's
# `replace github.com/skrptiq/engine => ../skrptiq-app/engine` resolves to it.
# `git archive` keeps this read-only on the app checkout -- no worktree, no
# branch switch, nothing a concurrently-working sibling repo would notice.
#
# HEAD, not the working tree: the question is whether what we would release
# builds, not whether an in-progress edit does.
if ! command -v go >/dev/null 2>&1; then
  fatal_lookup "go is not on PATH — the pin build cannot be verified, and an unverified pin is not a working pin"
fi

PINTMP="$(mktemp -d)"
trap 'rm -rf "$PINTMP"' EXIT
mkdir -p "$PINTMP/skrptiq-app" "$PINTMP/skrptiq-cli"

git -C "$APP_CHECKOUT" archive "$PIN" engine 2>/dev/null | tar -x -C "$PINTMP/skrptiq-app" \
  || fatal_lookup "could not extract engine/ at ${PIN:0:7} from the app checkout"
[ -f "$PINTMP/skrptiq-app/engine/go.mod" ] \
  || fatal_lookup "engine/go.mod absent at ${PIN:0:7} — the pin does not name a revision with an engine"
git -C "$REPO_ROOT" archive HEAD 2>/dev/null | tar -x -C "$PINTMP/skrptiq-cli" \
  || fatal_lookup "could not archive this repo at HEAD"

if BUILD_OUT="$(go build -C "$PINTMP/skrptiq-cli" ./... 2>&1)"; then
  note "CHECK 3 ok      — HEAD compiles against the pinned engine ${PIN:0:7}."
else
  FAILED=1
  err "THE PINNED ENGINE DOES NOT BUILD: this repo at HEAD does not compile against ${PIN:0:7}, the revision .engine-pin names."
  err "release.yml checks the engine out at that pin, so EVERY release build fails at the Build step — there is no releasable commit on this branch."
  err "ci.yml builds against app main and would not see this: main has the symbols, the pin does not."
  while IFS= read -r line; do [ -n "$line" ] && err "  $line"; done <<< "$BUILD_OUT"
fi

note ""
if [ "$FAILED" -ne 0 ]; then
  err "The App -> CLI release chain is not delivering. This is an incident, not a no-op."
  exit 1
fi
note "Engine release chain: delivering."
exit 0
