# GH#630 Plan — `skrptiq scan` resolves dep-provided slugs

**Status:** awaiting orchestrator approval
**Closure gate:** `scripts/cli-scan.sh examples/roadmap-generation-flow` (Hub) returns 0 errors with the dep-referenced manifest. Not "Go tests pass".

## 1. Scope

Teach `skrptiq scan` to treat `workflow.execution[].skill` / `workflow.execution[].prompt` refs as resolved when the slug is provided by a declared dep in the package manifest's `dependencies:` block.

Engine prereq is in place: `parse.Package.Dependencies []DependencyRef` (SKRPT-INTEGRATION §2.0.1, shipped under GH#612) already gives the scanner typed access to the dep block. Each ref carries `ID` (`hub-shared/<slug>`), `Version`, and `Checksum` — enough to fetch and verify.

Surfaces touched (CLI repo only):
- `internal/scan/scan.go` — add dep-resolution step before workflow validation.
- New `internal/scan/depresolver/` — fetch + cache + verify the slug-list per dep.
- `cmd/skrptiq/scan.go` (or wherever the flag is registered) — add `--no-resolve-deps`.
- New tests under `internal/scan/`.

Out of scope (per GH#630 body): engine runtime edge resolution, per-consumer overlay (GH#626), parameterised skrpts (GH#627), Hub publish-side workaround.

## 2. Resolution algorithm — confirmed

Hub agent's three-tier proposal, adopted as stated:

1. Slug exists in the local package → resolved.
2. Else slug is provided by ANY declared dep in `manifest.dependencies` → resolved.
3. Else → ERROR (`workflow.execution_skill_missing` / `workflow.execution_prompt_missing`, unchanged codes).

No contest. The error codes are preserved so consumers diffing scan output don't see code churn; only the trigger condition narrows.

## 3. Dep-node discovery — hybrid fetch + cache, **with checksum verification**

Recommend the Hub agent's hybrid path, with one refinement: **verify the manifest-declared dep checksum against what Hub returns**. Without that, the scanner trusts whatever bytes the catalogue serves, which is exactly the kind of phantom-dependency hole CTO §5 warned against.

### Endpoint contract (coordinate with Hub agent before approval)

Scan needs, per `<slug>@<version>`:
- the bundle's content checksum (to compare against `DependencyRef.Checksum`)
- the list of node slugs the bundle provides

Two options for Hub side; either is acceptable:

- **Option A (preferred):** extend `GET ${hubUrl}/api/shared/<slug>/<v>/metadata` to include `nodes: [{id, type}]` and `checksum: "sha256:..."`. Single round-trip per dep. Reuses the existing `Cache-Control: public, max-age=31536000, immutable` header.
- **Option B:** add `GET ${hubUrl}/api/shared/<slug>/<v>/nodes` returning `{checksum, nodes:[{id, type}]}`. Same shape, separate endpoint.

**This needs a Hub-agent confirmation comment on GH#630 before I implement.** Plan locks the contract, then both sides build to it. If Hub prefers a different shape, plan amends.

### Cache

- Location: `~/.skrptiq/cache/dep-nodes.json` (single file; JSON map keyed by `<id>@<version>@<checksum>`).
- TTL: indefinite. The `<checksum>` segment in the key means any drift invalidates the entry naturally — no clock-based expiry needed, matches the immutable Cache-Control intent.
- Schema (each entry): `{ "id": "...", "version": "...", "checksum": "...", "nodes": [{"id": "...", "type": "..."}], "fetched_at": "..." }`. `fetched_at` is informational only (debugging stale-cache reports), not used for invalidation.
- Concurrency: read-modify-write under a file lock (`flock`) so parallel `skrptiq scan` invocations don't shred the file.
- Failure mode: cache read errors are logged and treated as cache miss. Cache write errors are logged WARN and don't fail the scan (the resolution itself succeeded; the cache is an optimisation).

### Verification (the trust-chain step)

For each `DependencyRef` in the manifest:

1. Cache lookup by `<id>@<version>@<checksum>`. Hit → use cached `nodes`. Miss → fetch.
2. Fetch metadata/nodes endpoint. If Hub-returned `checksum != DependencyRef.Checksum` → **ERROR** `dependency.checksum_mismatch` (does not write to cache; surfaces as scan failure).
3. On match, write entry to cache and use `nodes` for resolution.

This makes the scanner enforce what CTO §F2.1 calls the "manifest-declared identity/version/checksum exactly" rule at scan time, not just at install time.

### Failure modes (all surface as scan ERROR, not warning)

| Code | Trigger |
|---|---|
| `dependency.fetch_failed` | Network error or non-2xx fetching metadata in strict mode |
| `dependency.checksum_mismatch` | Hub-returned checksum ≠ manifest DependencyRef.Checksum |
| `dependency.unresolved_slug` | Slug not in local package AND not in any dep's node list |
| `workflow.execution_skill_missing` / `..._prompt_missing` | (unchanged code) emitted only after dep resolution exhausted |

The last two are not the same error: `unresolved_slug` is the dep-aware case ("you declared dep X, X doesn't provide this slug"); the existing `execution_*_missing` codes stay for the no-deps-at-all case so existing scan output for legacy bundles is unchanged.

## 4. Two-mode scanner — confirms CTO review §5

Per CTO `shared-object-publishing-gh609-2026-05-26.md` §5:

- **Default (strict, dep-resolving):** the behaviour above. Required for `publish-skrpt.mjs` CI gates. This is the new shipped behaviour for `skrptiq scan` with no flag.
- **`--no-resolve-deps` (structure-only):** validates the shape of the `dependencies:` block (each entry has id/version/checksum, version is non-empty, etc.) but does NOT fetch from Hub. Workflow-execution refs to slugs not in the local package are accepted as "potentially dep-provided" — no error, no fetch. For local-dev / offline authoring.

`--no-resolve-deps` is **more** permissive than today's behaviour, not less. The old "everything must be local" path is removed entirely, no parallel mode, no feature flag — per the standing no-transitional-fallbacks rule. Hub's `publish-skrpt.mjs` ungated calls `skrptiq scan` without the flag and gets strict-mode resolution; that's the publish gate.

## 5. Test coverage

New test fixtures under `internal/scan/testdata/`:

- `dep-referenced-valid/` — small consumer bundle with `dependencies:` listing one dep, workflow execution refs to dep-provided slugs. **Strict mode + mocked Hub → 0 errors.** This is the GH#630 regression test.
- `dep-referenced-missing-slug/` — same shape, but workflow refs a slug neither in local nor in dep's node list → ERROR `dependency.unresolved_slug`.
- `dep-checksum-mismatch/` — manifest declares checksum X, mock Hub returns checksum Y → ERROR `dependency.checksum_mismatch`.
- `dep-fetch-fails/` — mock Hub returns 503 → ERROR `dependency.fetch_failed`.
- `no-deps-bundle/` — existing legacy package, no `dependencies:` block, workflow refs nonexistent slug → ERROR `workflow.execution_skill_missing` (unchanged behaviour — proves no regression for legacy bundles).

Plus unit tests for `depresolver/`:
- Cache hit / cache miss / cache corruption (treat as miss).
- File-lock concurrency (two goroutines fetching same dep → one network call).
- `--no-resolve-deps`: same fixtures, no network mock — passes the valid case, structurally validates the dep block.

Mock Hub: `httptest.Server` returning canned metadata responses. No live Hub dependency in CI.

## 6. Closure criterion (K-035) — explicit

The gate is not `go test ./...`. The gate is:

> Hub's `scripts/cli-scan.sh examples/roadmap-generation-flow` returns 0 errors against the dep-referenced manifest (the exact halt state captured in GH#630 "Reproducible halt state").

I'll request the Hub agent re-runs the Step 3 pilot reproduction against a pre-built binary from my PR branch and reports the result on the GH#630 thread before merge.

## 7. No transitional fallbacks — explicit

- Old "package-local-only" resolution is **deleted**, not parallel-pathed. No `--strict-deps` flag, no env-var opt-in, no settings toggle.
- `--no-resolve-deps` exists for local dev only; it is structurally **weaker** than strict mode (skips fetch). It does not preserve old behaviour — the old behaviour erroneously errored on dep-provided slugs, and that bug is what GH#630 deletes.
- If the new resolution path is unreliable (e.g. Hub endpoint not stable), it is not ready to ship and the plan amends. There is no "old behaviour as fallback" hatch.

## 8. Workflow

PR, not direct-to-main. Adds new network surface, persistent cache file in user's `~/.skrptiq/cache/`, new flag, new error codes, and a Hub endpoint contract dependency. Substantial enough to warrant explicit review per the PR-workflow rule.

## 9. Dependencies on other agents

- **Hub agent must confirm endpoint shape (Option A or B) on GH#630 before I implement.** Without that, I'd be guessing.
- Hub agent re-runs `scripts/cli-scan.sh examples/roadmap-generation-flow` against my PR binary before merge as the K-035 sign-off.

## 10. Open questions for orchestrator

- The hybrid fetch+cache assumes the metadata endpoint is reachable from CI. Hub's `cli-scan.sh` runs in GH Actions — is the Hub catalogue reachable from that environment? If not, the strict-mode publish gate needs a different story (e.g. ship a checked-in `dep-nodes.json` snapshot in the Hub repo, scanner reads from `--dep-cache <path>` instead of fetching). Flag before implementation if this is a known constraint.
- Confirm PR workflow over direct-to-main for this.
