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
# PIN_FILE's override exists for two reasons, named here because an unnamed
# hatch gets removed by the next reader (GH#1192):
#
#   1. DELEGATION — this script passes PIN_FILE through to
#      `engine-pin-build.sh`, which is how GH#1118's one-authority split works
#      at all: one implementation of "does HEAD compile against the pin", two
#      callers (this script and ci.yml's `engine: release pin` job).
#   2. TESTABILITY — it is how both directions of a pin check get exercised
#      without mutating the repo. `PIN_FILE=/tmp/oldpin` is what proved the new
#      check fails on the incident's pin (18c63f01) and passes on the current
#      one, with no commit in between.
#
# AND ITS HAZARD, recorded beside its purpose: this is also the mechanism that
# makes the pin's SUBJECT ambiguous. The default reads the WORKING CHECKOUT, and
# this script asks two questions that want two different pins — see the
# provenance block below.
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

# --- the pin's PROVENANCE (GH#1192) ----------------------------------------
#
# ONE SCRIPT, TWO SUBJECTS, ONE VARIABLE — that is the defect, stated at the
# level it actually lives at:
#
#   checks 1 and 2 ask "is the chain DELIVERING". Their subject is the FLEET:
#     what main pins, what app released. They want origin/main's pin.
#   check 3 asks "does THIS REF compile". Its subject is this ref, and it must
#     keep the working pin, because `git archive HEAD` is compiled against that
#     same value. Relocating that read would put two subjects in one comparison.
#
# In CI the two coincide: the workflow runs on main and the checkout IS
# origin/main, so the value is correct BY ACCIDENT OF ENVIRONMENT. That is why
# this survived — the only configuration where it is wrong is a local run, which
# is the one with no CI run to go red. Five months of correct behaviour was not
# evidence.
#
# So: say where the pin came from, and when the working pin is not the one main
# carries, REFUSE TO CLAIM on checks 1 and 2 rather than answering from the
# wrong tree. Not a failure — an explicit "cannot claim".
#
# NO NETWORK IS REQUIRED for any of this. `git show origin/main:.engine-pin`
# reads the remote-tracking ref, which is local. A correctness check that needs
# connectivity is one people learn to skip.
WORK_PIN="$(tr -d '[:space:]' < "$PIN_FILE" 2>/dev/null || echo '')"
MAIN_PIN="$(git -C "$REPO_ROOT" show origin/main:.engine-pin 2>/dev/null | tr -d '[:space:]' || echo '')"
PIN_SOURCE="the working checkout"
[ "$PIN_FILE" = "$REPO_ROOT/.engine-pin" ] || PIN_SOURCE="PIN_FILE override ($PIN_FILE)"

# CAN_CLAIM_DELIVERY gates checks 1 and 2 only. It defaults to 1 and is only
# cleared on a POSITIVE divergence — an unreadable origin/main is reported, not
# treated as divergence, because the common cause is a shallow CI checkout with
# no remote-tracking ref and failing closed there would redden a correct chain.
CAN_CLAIM_DELIVERY=1
PIN_NOTE=""
if [ -z "$MAIN_PIN" ]; then
  PIN_NOTE="origin/main's pin is unreadable here (no remote-tracking ref) — provenance not cross-checked"
elif [ "$WORK_PIN" != "$MAIN_PIN" ]; then
  CAN_CLAIM_DELIVERY=0
  PIN_NOTE="DIVERGES from origin/main's pin ${MAIN_PIN:0:7}"
fi

note "App newest release : ${REL_TAG}  (published ${REL_PUB}, ${AGE_HOURS}h ago)"
note "CLI engine pin     : ${WORK_PIN:-<unreadable>}  (read from ${PIN_SOURCE})"
if [ -n "$PIN_NOTE" ]; then
  note "                     ${PIN_NOTE}"
fi
# A no-network comparison is only as good as the last fetch, so date the ref
# rather than implying it is current. One line; the difference between a
# comparison and a claim.
if [ -n "$MAIN_PIN" ]; then
  note "origin/main         : last fetched $(git -C "$REPO_ROOT" log -1 --format=%ad --date=iso origin/main 2>/dev/null || echo '<unknown>')"
fi
note ""

IN_FLIGHT=0
[ "$AGE_HOURS" -lt "$GRACE_HOURS" ] && IN_FLIGHT=1
FAILED=0

# --- checks 1 and 2: are they ANSWERABLE from this tree? (GH#1192) ----------
#
# Both ask about the FLEET — what main pins, what app released. A working
# checkout whose pin main does not carry cannot answer that, so this refuses to
# claim rather than answering from the wrong tree.
#
# THREE-VALUED, deliberately: ok / failed / cannot-claim. A check that refuses to
# answer is worth more than one that answers from the wrong subject, and the
# refusal must not be reported as a pass — `CANNOT CLAIM` is not `ok`.
#
# It does NOT set FAILED: the chain is not broken by a local tree being out of
# date. The exit code below stays 0 so a developer's local run does not read as
# an incident, and the lines say plainly why no verdict was reached.
if [ "$CAN_CLAIM_DELIVERY" -eq 0 ]; then
  note "CHECK 1 CANNOT CLAIM — this checkout's pin ${WORK_PIN:0:7} is not the pin origin/main carries (${MAIN_PIN:0:7})."
  note "CHECK 2 CANNOT CLAIM — 'is the chain delivering' is a question about the FLEET, and this tree is not what the fleet pins."
  note "                       Not a failure and not a pass. Run it on a checkout of main, or in CI, to get a verdict."
  note ""
else

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

fi  # end CAN_CLAIM_DELIVERY

# --- check 3: does HEAD still compile against the pinned engine? -----------
#
# OUTSIDE the CAN_CLAIM_DELIVERY gate, deliberately (GH#1192). This is the
# pre-commit question — "would what I am about to push release?" — and its
# subject is THIS ref, so a working pin is the right pin for it and a diverged
# one changes nothing about its validity. It also stays offline: a correctness
# check that needs connectivity is one people learn to skip.
#
# GH#1118 — DELEGATED, not implemented here. `scripts/engine-pin-build.sh` owns
# this rule and `ci.yml` calls the same file per-PR, so there is ONE
# implementation and two callers rather than two copies that drift (GH#1058).
#
# The split exists because CI must be able to ask THIS question without asking
# checks 1 and 2: those query the releases API and are about whether app's chain
# is delivering, so running them per-PR would fail cli's PRs whenever app had
# simply not released recently.
if PIN_OUT="$(APP_CHECKOUT="$APP_CHECKOUT" PIN_FILE="$PIN_FILE" "$REPO_ROOT/scripts/engine-pin-build.sh" 2>&1)"; then
  # WORK_PIN, not PIN: `PIN` is assigned inside the delivery-gated region above,
  # so it is unset whenever checks 1 and 2 refused to claim — and `set -u` then
  # kills the script before check 3 can report. Found by running the
  # diverged-pin-that-still-builds combination rather than the two obvious cases
  # (GH#1192). WORK_PIN is assigned unconditionally with the provenance.
  note "CHECK 3 ok      — HEAD compiles against the pinned engine ${WORK_PIN:0:7}."
else
  PIN_RC=$?
  # Exit 2 is the script's fail-closed case (it could not establish an answer);
  # keep the watchdog's existing behaviour of treating that as unverifiable
  # rather than as a clean failure.
  if [ "$PIN_RC" -ge 2 ]; then
    while IFS= read -r line; do [ -n "$line" ] && err "  $line"; done <<< "$PIN_OUT"
    fatal_lookup "scripts/engine-pin-build.sh could not establish whether the pin builds"
  fi
  FAILED=1
  while IFS= read -r line; do [ -n "$line" ] && echo "$line" >&2; done <<< "$PIN_OUT"
fi

note ""
if [ "$FAILED" -ne 0 ]; then
  err "The App -> CLI release chain is not delivering. This is an incident, not a no-op."
  exit 1
fi
# GH#1192 — the SUMMARY must not claim what the checks refused to claim. With a
# diverged pin, checks 1 and 2 reached no verdict, so "delivering" would be the
# script asserting fleet health it explicitly declined to measure — the exact
# defect this change exists to remove, reintroduced in the last line.
#
# Caught by testing the combination (pin diverges AND check 3 passes); the two
# obvious cases both hid it, because one fails on check 3 and the other has no
# divergence.
if [ "$CAN_CLAIM_DELIVERY" -eq 0 ]; then
  note "Engine release chain: NO VERDICT — this tree's pin is not the fleet's. Check 3 passed, which"
  note "is a statement about THIS ref only. Run on a checkout of main, or in CI, to grade the chain."
  exit 0
fi
note "Engine release chain: delivering."
exit 0
