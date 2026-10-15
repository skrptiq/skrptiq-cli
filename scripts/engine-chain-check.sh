#!/usr/bin/env bash
#
# engine-chain-check.sh — assert that the App -> CLI auto-release chain
# (GH#539) is actually delivering, and FAIL LOUDLY when it is not.
#
# WHY THIS EXISTS (GH#1079).
#
# The chain never fired once. `dispatch-cli-after-release.yml` in the App was
# switched to `on: release: published` by GH#564 on 2026-05-17; the App's
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
# TWO INDEPENDENT CHECKS. They fail for different reasons on purpose; either
# one alone would have missed a real case.
#
#   1. DISPATCH NOT SENT — for the App's newest release, did the App's dispatch
#      workflow produce any run at all? This is the check that catches the
#      actual GH#1079 fault, and it fires even when the release carried no
#      engine change (where check 2 is legitimately silent).
#
#   2. SHIPPED ENGINE IS BEHIND — does `engine/` differ between our .engine-pin
#      and the App's newest release tag? This catches delivery failing anywhere
#      downstream of the dispatch (receiver, tag job, build) and is the check
#      that speaks to what we actually ship.
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
#   PIN_FILE      default <repo root>/.engine-pin
#   APP_CHECKOUT  a full checkout of APP_REPO (tags + history) for check 2
#   GRACE_HOURS   default 6 -- a release younger than this is still in flight,
#                 so a missing dispatch is reported but not failed

set -uo pipefail

APP_REPO="${APP_REPO:-skrptiq/skrptiq-app}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PIN_FILE="${PIN_FILE:-$REPO_ROOT/.engine-pin}"
GRACE_HOURS="${GRACE_HOURS:-6}"
DISPATCH_WF="dispatch-cli-after-release.yml"

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

# --- check 1: was a dispatch sent at all? -----------------------------------
RUNS="$(gh run list --repo "$APP_REPO" --workflow "$DISPATCH_WF" --limit 100 \
  --json createdAt --jq "[.[] | select(.createdAt >= \"${REL_PUB}\")] | length" 2>/dev/null)"
case "$RUNS" in
  ''|*[!0-9]*) fatal_lookup "could not list runs of ${DISPATCH_WF} in ${APP_REPO}" ;;
esac

if [ "$RUNS" -gt 0 ]; then
  note "CHECK 1 ok      — ${DISPATCH_WF} produced ${RUNS} run(s) for ${REL_TAG}."
elif [ "$IN_FLIGHT" -eq 1 ]; then
  note "CHECK 1 pending — no dispatch run yet, but ${REL_TAG} is only ${AGE_HOURS}h old (grace ${GRACE_HOURS}h)."
else
  FAILED=1
  err "CHAIN BROKEN AT THE SENDER: ${APP_REPO} published ${REL_TAG} ${AGE_HOURS}h ago and ${DISPATCH_WF} has not run even once."
  err "The App's release did not start its own dispatch workflow — so no dispatch reached skrptiq-cli, and no CLI release can exist for this engine."
  err "Known cause (GH#1079): a release created under the default GITHUB_TOKEN raises no workflow-triggering event, so 'on: release: published' never fires. Fix belongs in ${APP_REPO}."
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
    err "SHIPPED ENGINE IS STALE: ${CHANGED} file(s) under engine/ differ between our pin ${PIN:0:7} and the App's released ${REL_TAG}."
    err "Every CLI binary we ship is built against ${PIN:0:7}. A scanner on a stale engine grades against rules the shipped engine no longer holds — it passes content the current engine refuses (GH#1079), and it fails permissive."
    err "Do not fix this by hand-bumping .engine-pin: that ships a current engine through a broken pipe and hides the outage. Repair the chain."
  fi
fi

note ""
if [ "$FAILED" -ne 0 ]; then
  err "The App -> CLI release chain is not delivering. This is an incident, not a no-op."
  exit 1
fi
note "Engine release chain: delivering."
exit 0
