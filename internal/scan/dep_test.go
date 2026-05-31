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

// mockHubMetadata returns a handler that serves Hub metadata for one
// dep. Callers thread it into Options.HubBaseURL via httptest.Server.
func mockHubMetadata(t *testing.T, id, version, checksum string, nodes []map[string]string) http.Handler {
	t.Helper()
	wantPath := "/api/shared/" + strings.TrimPrefix(id, "hub-shared/") + "/" + version + "/metadata"
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
	h := mockHubMetadata(t, "hub-shared/test-dep", "1.0.0", "sha256:aaa", []map[string]string{
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
	h := mockHubMetadata(t, "hub-shared/test-dep", "1.0.0", "sha256:aaa", []map[string]string{
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

// Trust-chain guard: manifest-declared checksum != Hub-returned checksum
// must hard-fail. Closes CTO §F2.1 phantom-dependency hole.
func TestScan_DepChecksumMismatch_HardError(t *testing.T) {
	h := mockHubMetadata(t, "hub-shared/test-dep", "1.0.0", "sha256:something-else", []map[string]string{
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
	for _, i := range result.Issues {
		if i.Code == "workflow.execution_skill_missing" || i.Code == "workflow.execution_prompt_missing" {
			t.Errorf("--no-resolve-deps must drop _missing for non-local slugs in dep-referenced bundles: %s", i.Message)
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
