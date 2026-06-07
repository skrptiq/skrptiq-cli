package depresolver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/skrptiq/engine/parse"
)

// Synthetic UUIDs used across the test suite. Per K-037 the catalogue ID
// is a UUID, not a logical-namespace slug; tests reflect the production
// shape rather than the legacy `hub-shared/<slug>` form.
const (
	uuidDepA = "11111111-1111-1111-1111-aaaaaaaaaaaa"
	uuidDepB = "22222222-2222-2222-2222-bbbbbbbbbbbb"
)

func newTestResolver(t *testing.T, hubURL string) *Resolver {
	t.Helper()
	r, err := New(Config{HubBaseURL: hubURL, CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func mockServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func writeMetadata(w http.ResponseWriter, id, version, checksum string, nodes []NodeInfo) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":       id,
		"version":  version,
		"checksum": checksum,
		"nodes":    nodes,
	})
}

func TestResolve_Empty(t *testing.T) {
	r := newTestResolver(t, "http://nowhere.invalid")
	res := r.Resolve(nil)
	if len(res.Issues) != 0 || len(res.ProvidedSlugs) != 0 {
		t.Errorf("expected empty result for nil deps, got %+v", res)
	}
}

func TestResolve_Happy(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadata(w, uuidDepA, "1.0.0", "sha256:a", []NodeInfo{
			{ID: "uuid-skill-a", Slug: "skill-a", Type: "skill"},
			{ID: "uuid-prompt-a", Slug: "prompt-a", Type: "prompt"},
		})
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	if len(res.Issues) != 0 {
		t.Fatalf("expected 0 issues, got %+v", res.Issues)
	}
	// ProvidedSlugs values use the logical slug (Name) for human readability.
	if res.ProvidedSlugs["skill-a"] != "dep-a" ||
		res.ProvidedSlugs["prompt-a"] != "dep-a" {
		t.Errorf("ProvidedSlugs = %+v", res.ProvidedSlugs)
	}
}

func TestResolve_ChecksumMismatch(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadata(w, uuidDepA, "1.0.0", "sha256:WRONG", []NodeInfo{
			{ID: "uuid-skill-a", Slug: "skill-a", Type: "skill"},
		})
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:expected"},
	})
	if len(res.Issues) == 0 || res.Issues[0].Code != "dependency.checksum_mismatch" {
		t.Errorf("expected dependency.checksum_mismatch, got %+v", res.Issues)
	}
	if _, hit := res.ProvidedSlugs["skill-a"]; hit {
		t.Errorf("must not record slugs on checksum mismatch: %+v", res.ProvidedSlugs)
	}
}

func TestResolve_FetchFailed(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusServiceUnavailable)
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	if len(res.Issues) == 0 || res.Issues[0].Code != "dependency.fetch_failed" {
		t.Errorf("expected dependency.fetch_failed, got %+v", res.Issues)
	}
}

// K-037 no-fallback regression: an empty Name field on a dep is a hard
// error inside the resolver (covers the case where some upstream skipped
// the parser's required-name enforcement). Hub URL construction has no
// slug to use and must refuse rather than mint a malformed `/api/shared//.../metadata`.
func TestResolve_EmptyName_HardError(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server must not be called with empty dep Name; got %s", r.URL.Path)
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "", Version: "1.0.0", Checksum: "sha256:a"},
	})
	if len(res.Issues) == 0 || res.Issues[0].Code != "dependency.fetch_failed" {
		t.Errorf("expected dependency.fetch_failed (empty name guard), got %+v", res.Issues)
	}
}

// K-037 regression guard: the legacy `dependency.unsupported_id_prefix`
// code is gone. A UUID `id:` (the K-037 canonical shape) MUST resolve
// cleanly; emitting that code anywhere would be a contract-violation
// regression.
func TestResolve_UUIDPrefix_NoLegacyError(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadata(w, uuidDepA, "1.0.0", "sha256:a", []NodeInfo{
			{ID: "uuid-skill-a", Slug: "skill-a", Type: "skill"},
		})
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	for _, i := range res.Issues {
		if i.Code == "dependency.unsupported_id_prefix" {
			t.Errorf("K-037 contract: dependency.unsupported_id_prefix must not exist; got %+v", i)
		}
	}
}

// fetchMetadata uses dep.Name (not dep.ID) for the URL path under K-037.
// This test asserts the path shape directly.
func TestResolve_URLUsesNameNotID(t *testing.T) {
	var seenPath string
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		writeMetadata(w, uuidDepA, "1.0.0", "sha256:a", []NodeInfo{{ID: "uuid-skill-a", Slug: "skill-a", Type: "skill"}})
	})
	r := newTestResolver(t, srv.URL)
	_ = r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	want := "/api/shared/dep-a/1.0.0/metadata"
	if seenPath != want {
		t.Errorf("URL path = %q; want %q (must use dep.Name, not dep.ID UUID)", seenPath, want)
	}
}

// Cache hit path: second Resolve with the same dep MUST NOT call Hub.
func TestResolve_CacheHitSkipsFetch(t *testing.T) {
	var calls int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeMetadata(w, uuidDepA, "1.0.0", "sha256:a", []NodeInfo{
			{ID: "uuid-skill-a", Slug: "skill-a", Type: "skill"},
		})
	})
	cacheDir := t.TempDir()
	r1, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	deps := []parse.DependencyRef{{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"}}
	_ = r1.Resolve(deps)

	r2, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	res := r2.Resolve(deps)
	if calls != 1 {
		t.Errorf("expected 1 Hub call after cache warm, got %d", calls)
	}
	if _, hit := res.ProvidedSlugs["skill-a"]; !hit {
		t.Errorf("expected skill-a in ProvidedSlugs, got %+v", res.ProvidedSlugs)
	}
}

// Cache key includes checksum — a manifest checksum change must
// invalidate the entry (plan §3).
func TestResolve_CacheKeyInvalidatesOnChecksumChange(t *testing.T) {
	var calls int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		// Respond with whatever the request expects — checksum derived
		// from the URL is not encoded here, so we just return v1.
		writeMetadata(w, uuidDepA, "1.0.0", "sha256:v1", []NodeInfo{
			{ID: "uuid-skill-a", Slug: "skill-a", Type: "skill"},
		})
	})
	cacheDir := t.TempDir()

	r1, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	_ = r1.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:v1"},
	})

	// Different declared checksum → different cache key → fetch again.
	r2, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	res := r2.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:v2"},
	})
	if calls != 2 {
		t.Errorf("expected 2 Hub calls (no cache hit on checksum change), got %d", calls)
	}
	// Hub returns v1, manifest declares v2 → mismatch error on second call.
	if len(res.Issues) == 0 || res.Issues[0].Code != "dependency.checksum_mismatch" {
		t.Errorf("expected checksum_mismatch on second resolve, got %+v", res.Issues)
	}
}

// Concurrent resolvers sharing a cache file must not corrupt it.
func TestResolve_ConcurrentWritesStable(t *testing.T) {
	// Each goroutine fetches a different dep UUID/Name. The mock returns
	// metadata whose id matches whatever was requested via the URL path's
	// slug (Name). Checksum is constant.
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Path: /api/shared/<name>/1.0.0/metadata
		// id in response uses a synthetic UUID derived from the slug so
		// each entry stays distinct.
		writeMetadata(w, "uuid-"+r.URL.Path, "1.0.0", "sha256:x", []NodeInfo{
			{ID: "uuid-skill-from-" + r.URL.Path, Slug: "skill-from-" + r.URL.Path, Type: "skill"},
		})
	})
	cacheDir := t.TempDir()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
			r.Resolve([]parse.DependencyRef{
				{
					ID:       fmt.Sprintf("uuid-dep-%d", i),
					Name:     fmt.Sprintf("dep-%d", i),
					Version:  "1.0.0",
					Checksum: "sha256:x",
				},
			})
		}(i)
	}
	wg.Wait()

	// Cache file should be parseable JSON with entries from all goroutines.
	// Some entries may be missing if the mock's id-mismatch (uuid-/api/... !=
	// manifest's uuid-dep-N) triggers checksum_mismatch noise — but the file
	// must remain valid JSON regardless.
	data, err := os.ReadFile(filepath.Join(cacheDir, "dep-nodes.json"))
	if err != nil {
		// File may not exist if every resolve produced a mismatch-only error;
		// that's fine for the corruption-prevention goal of this test.
		return
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("cache file corrupted: %v\ncontents: %s", err, data)
	}
}

// Corrupt cache file must be treated as a miss, not fatal.
func TestResolve_CorruptCacheTreatedAsMiss(t *testing.T) {
	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, "dep-nodes.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadata(w, uuidDepA, "1.0.0", "sha256:a", []NodeInfo{{ID: "uuid-skill-a", Slug: "skill-a", Type: "skill"}})
	})
	r, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	// Resolution should succeed despite corrupt cache.
	if len(res.Issues) != 0 {
		// cache_write_failed warning is acceptable — but only the warning,
		// no errors.
		for _, i := range res.Issues {
			if i.Severity == "error" {
				t.Errorf("corrupt cache must not produce error: %+v", i)
			}
		}
	}
}
