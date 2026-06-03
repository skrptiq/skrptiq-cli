package scan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Shared synthetic UUID used by every test-dep manifest fixture +
// mockHubMetadata call. Per K-037 the dep's catalogue id is a UUID;
// the slug (`test-dep`) lives separately on Name + appears in URL paths.
const testDepUUID = "33333333-3333-3333-3333-cccccccccccc"

// mockHubMetadata returns a handler that serves Hub metadata for one
// dep. Callers thread it into Options.HubBaseURL via httptest.Server.
//
// Per K-037: `id` is the catalogue UUID (returned verbatim in the
// metadata body); `slug` is the logical name appearing in the URL path
// `/api/shared/<slug>/<version>/metadata`. Both must match the
// consumer manifest's `dependencies[].id` (UUID) and `.name` (slug)
// respectively.
func mockHubMetadata(t *testing.T, id, slug, version, checksum string, nodes []map[string]string) http.Handler {
	t.Helper()
	wantPath := "/api/shared/" + slug + "/" + version + "/metadata"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       id,
			"version":  version,
			"checksum": checksum,
			"nodes":    nodes,
		})
	})
}

func runScanWithDeps(t *testing.T, path string, h http.Handler, opts Options) ScanResult {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	if opts.HubBaseURL == "" {
		opts.HubBaseURL = srv.URL
	}
	if opts.DepCacheDir == "" {
		opts.DepCacheDir = t.TempDir()
	}
	var buf bytes.Buffer
	RunWithOptions(path, true, opts, &buf)
	var result ScanResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON parse failed: %v\noutput: %s", err, buf.String())
	}
	return result
}

// GH#630 — the headline test: a consumer bundle with a dep-referenced
// workflow.execution skill/prompt that resolves cleanly via the
// declared dep. Without the GH#630 fix this fails with 2 errors
// (workflow.execution_skill_missing + workflow.execution_prompt_missing).
func TestScan_DepReferencedValid_NoErrors(t *testing.T) {
	h := mockHubMetadata(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa", []map[string]string{
		{"id": "dep-skill", "type": "skill"},
		{"id": "dep-prompt", "type": "prompt"},
	})
	result := runScanWithDeps(t, testdataPath("dep-referenced-valid"), h, Options{})

	if result.ErrorCount > 0 {
		t.Errorf("expected 0 errors, got %d", result.ErrorCount)
		for _, i := range result.Issues {
			t.Logf("  %s [%s]: %s (%s)", i.Severity, i.Code, i.Message, i.File)
		}
	}
	for _, i := range result.Issues {
		if i.Code == "workflow.execution_skill_missing" || i.Code == "workflow.execution_prompt_missing" {
			t.Errorf("dep-provided slug must not produce %s: %s", i.Code, i.Message)
		}
	}
}

// GH#630 — slug neither local nor in any declared dep's node list must
// surface dependency.unresolved_slug (not the legacy workflow.execution_*_missing
// code, which is reserved for no-deps-at-all bundles).
func TestScan_DepReferencedMissingSlug_UnresolvedDepCode(t *testing.T) {
	h := mockHubMetadata(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa", []map[string]string{
		{"id": "dep-prompt", "type": "prompt"}, // dep-skill not provided
	})
	result := runScanWithDeps(t, testdataPath("dep-referenced-missing-slug"), h, Options{})

	found := false
	for _, i := range result.Issues {
		if i.Code == "dependency.unresolved_slug" && strings.Contains(i.Message, "nowhere-skill") {
			found = true
		}
		if i.Code == "workflow.execution_skill_missing" {
			t.Errorf("dep-referenced bundle must re-code _missing → unresolved_slug, got legacy code: %s", i.Message)
		}
	}
	if !found {
		t.Errorf("expected dependency.unresolved_slug for nowhere-skill, issues:")
		for _, i := range result.Issues {
			t.Logf("  [%s] %s", i.Code, i.Message)
		}
	}
}

// GH#634 — loops[].steps[] referencing dep-provided slugs must NOT
// produce workflow.loop_step_missing. The fourth dep-blind surface in
// the scanner family (after workflow.execution + connections-edge +
// Hub's own output_step). Closes the engine GH#636 trust chain on
// the CLI consumer side. K-035 reproduction: Hub's
// `scripts/cli-scan.sh examples/blog-refinement-loop` halt state.
func TestScan_DepReferencedLoopStep_NoErrors(t *testing.T) {
	h := mockHubMetadata(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa", []map[string]string{
		{"id": "dep-skill", "type": "skill"},
		{"id": "dep-prompt", "type": "prompt"},
	})
	result := runScanWithDeps(t, testdataPath("dep-referenced-loop-valid"), h, Options{})

	if result.ErrorCount > 0 {
		t.Errorf("expected 0 errors, got %d", result.ErrorCount)
		for _, i := range result.Issues {
			t.Logf("  %s [%s]: %s (%s)", i.Severity, i.Code, i.Message, i.File)
		}
	}
	for _, i := range result.Issues {
		if i.Code == "workflow.loop_step_missing" {
			t.Errorf("dep-provided slug must not produce workflow.loop_step_missing: %s", i.Message)
		}
	}
}

// GH#630 plan v2 — connections edge to a dep-provided slug must NOT
// produce scan.edge_target_unresolved (bridge-side three-tier tier 1).
// Confirmed regression-fix for the Hub K-035 fail at comment-4588102292.
func TestScan_DepReferencedValid_ConnectionsEdgeToDepResolves(t *testing.T) {
	h := mockHubMetadata(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa", []map[string]string{
		{"id": "dep-skill", "type": "skill"},
		{"id": "dep-prompt", "type": "prompt"},
	})
	result := runScanWithDeps(t, testdataPath("dep-referenced-valid"), h, Options{})

	for _, i := range result.Issues {
		if i.Code == "scan.edge_target_unresolved" {
			t.Errorf("connection target dep-skill must NOT error in strict mode: %s", i.Message)
		}
	}
}

// GH#630 plan v2 — connections edge to a non-local non-dep-provided
// slug must re-code to dependency.unresolved_slug (bridge-side
// three-tier tier 3), same code the workflow.execution path emits.
func TestScan_DepReferencedMissingSlug_ConnectionsEdgeRecodes(t *testing.T) {
	h := mockHubMetadata(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa", []map[string]string{
		{"id": "dep-prompt", "type": "prompt"},
	})
	result := runScanWithDeps(t, testdataPath("dep-referenced-missing-slug"), h, Options{})

	foundEdge := false
	for _, i := range result.Issues {
		if i.Code == "dependency.unresolved_slug" && strings.Contains(i.Message, "nowhere-edge") {
			foundEdge = true
		}
		if i.Code == "scan.edge_target_unresolved" {
			t.Errorf("dep-referenced bundle must re-code edge_target_unresolved → unresolved_slug, got legacy code: %s", i.Message)
		}
	}
	if !foundEdge {
		t.Errorf("expected dependency.unresolved_slug for connection target nowhere-edge")
		for _, i := range result.Issues {
			t.Logf("  [%s] %s", i.Code, i.Message)
		}
	}
}

// GH#630 plan v2 — legacy bundle (no dependencies: block) with an
// unresolved connection edge target must keep the legacy
// scan.edge_target_unresolved code. The bridge-side three-tier tier 4.
// invalid-package fixture has an existing broken connection edge.
func TestScan_NoDepsBlock_ConnectionsEdgePreservesLegacyCode(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("legacy bundle must not touch Hub; got request to %s", r.URL.Path)
	})
	result := runScanWithDeps(t, testdataPath("invalid-package"), h, Options{})

	for _, i := range result.Issues {
		if i.Code == "dependency.unresolved_slug" {
			t.Errorf("legacy bundle must not produce dependency.unresolved_slug: %s", i.Message)
		}
	}
	// And the legacy code MUST still fire for the broken edge — this
	// is the regression guard for legacy-bundle behaviour.
	if !hasIssueCode(result, "scan.edge_target_unresolved") {
		t.Error("legacy bundle with broken connection edge must keep scan.edge_target_unresolved code")
		for _, i := range result.Issues {
			t.Logf("  [%s] %s", i.Code, i.Message)
		}
	}
}

// Trust-chain guard: manifest-declared checksum != Hub-returned checksum
// must hard-fail. Closes CTO §F2.1 phantom-dependency hole.
func TestScan_DepChecksumMismatch_HardError(t *testing.T) {
	h := mockHubMetadata(t, testDepUUID, "test-dep", "1.0.0", "sha256:something-else", []map[string]string{
		{"id": "dep-skill", "type": "skill"},
	})
	result := runScanWithDeps(t, testdataPath("dep-checksum-mismatch"), h, Options{})

	if !hasIssueCode(result, "dependency.checksum_mismatch") {
		t.Errorf("expected dependency.checksum_mismatch, got:")
		for _, i := range result.Issues {
			t.Logf("  [%s] %s", i.Code, i.Message)
		}
	}
	if result.ErrorCount == 0 {
		t.Errorf("checksum mismatch must produce errors; got %d", result.ErrorCount)
	}
}

// Network failure must surface dependency.fetch_failed and fail the
// scan (strict mode).
func TestScan_DepFetchFails_HardError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	})
	result := runScanWithDeps(t, testdataPath("dep-fetch-fails"), h, Options{})

	if !hasIssueCode(result, "dependency.fetch_failed") {
		t.Errorf("expected dependency.fetch_failed, got:")
		for _, i := range result.Issues {
			t.Logf("  [%s] %s", i.Code, i.Message)
		}
	}
}

// --no-resolve-deps must accept the dep-referenced-valid fixture WITHOUT
// touching Hub. Proves the structural-only mode.
func TestScan_NoResolveDeps_AcceptsWithoutFetch(t *testing.T) {
	calls := 0
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})
	result := runScanWithDeps(t, testdataPath("dep-referenced-valid"), h, Options{NoResolveDeps: true})

	if calls != 0 {
		t.Errorf("--no-resolve-deps must not call Hub; got %d calls", calls)
	}
	// Plan v1 §4: "Workflow-execution refs to slugs not in the local
	// package are accepted as 'potentially dep-provided' — no error."
	// Plan v2 extends the same permissive treatment to the bridge
	// (connections edge) surface. Zero errors total for the
	// dep-referenced-valid fixture under --no-resolve-deps.
	if result.ErrorCount != 0 {
		t.Errorf("--no-resolve-deps must produce 0 errors on dep-referenced-valid; got %d", result.ErrorCount)
		for _, i := range result.Issues {
			t.Logf("  [%s] %s", i.Code, i.Message)
		}
	}
}

// Regression: a legacy bundle (no `dependencies:` block) with a missing
// workflow.execution skill ref must keep the historical
// workflow.execution_skill_missing code unchanged.
func TestScan_NoDepsBlock_PreservesLegacyMissingCode(t *testing.T) {
	// invalid-package has no dependencies: block and historically
	// produced workflow.execution-related errors. Run it without any
	// Hub mock — the resolver must not fire.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("legacy bundle must not touch Hub; got request to %s", r.URL.Path)
	})
	// We deliberately don't care about the precise legacy issue set,
	// only that we don't see the new dep code on a no-deps bundle.
	result := runScanWithDeps(t, testdataPath("invalid-package"), h, Options{})

	for _, i := range result.Issues {
		if i.Code == "dependency.unresolved_slug" {
			t.Errorf("legacy bundle must not produce dependency.unresolved_slug: %s", i.Message)
		}
	}
}

func hasIssueCode(r ScanResult, code string) bool {
	for _, i := range r.Issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

// Sanity helper — keeps the test file compileable even if other helpers
// are removed from scan_test.go later.
var _ = fmt.Sprintf
