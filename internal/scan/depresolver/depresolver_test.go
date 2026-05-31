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
		writeMetadata(w, "hub-shared/dep-a", "1.0.0", "sha256:a", []NodeInfo{
			{ID: "skill-a", Type: "skill"},
			{ID: "prompt-a", Type: "prompt"},
		})
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: "hub-shared/dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	if len(res.Issues) != 0 {
		t.Fatalf("expected 0 issues, got %+v", res.Issues)
	}
	if res.ProvidedSlugs["skill-a"] != "hub-shared/dep-a" ||
		res.ProvidedSlugs["prompt-a"] != "hub-shared/dep-a" {
		t.Errorf("ProvidedSlugs = %+v", res.ProvidedSlugs)
	}
}

func TestResolve_ChecksumMismatch(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadata(w, "hub-shared/dep-a", "1.0.0", "sha256:WRONG", []NodeInfo{
			{ID: "skill-a", Type: "skill"},
		})
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: "hub-shared/dep-a", Version: "1.0.0", Checksum: "sha256:expected"},
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
		{ID: "hub-shared/dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	if len(res.Issues) == 0 || res.Issues[0].Code != "dependency.fetch_failed" {
		t.Errorf("expected dependency.fetch_failed, got %+v", res.Issues)
	}
}

func TestResolve_UnsupportedIDPrefix(t *testing.T) {
	// Server should never be called.
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected fetch for non-hub-shared id: %s", r.URL.Path)
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: "elsewhere/foo", Version: "1.0.0", Checksum: "sha256:a"},
	})
	if len(res.Issues) == 0 || res.Issues[0].Code != "dependency.unsupported_id_prefix" {
		t.Errorf("expected dependency.unsupported_id_prefix, got %+v", res.Issues)
	}
}

// Cache hit path: second Resolve with the same dep MUST NOT call Hub.
func TestResolve_CacheHitSkipsFetch(t *testing.T) {
	var calls int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		writeMetadata(w, "hub-shared/dep-a", "1.0.0", "sha256:a", []NodeInfo{
			{ID: "skill-a", Type: "skill"},
		})
	})
	cacheDir := t.TempDir()
	r1, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	deps := []parse.DependencyRef{{ID: "hub-shared/dep-a", Version: "1.0.0", Checksum: "sha256:a"}}
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
		writeMetadata(w, "hub-shared/dep-a", "1.0.0", "sha256:v1", []NodeInfo{
			{ID: "skill-a", Type: "skill"},
		})
	})
	cacheDir := t.TempDir()

	r1, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	_ = r1.Resolve([]parse.DependencyRef{
		{ID: "hub-shared/dep-a", Version: "1.0.0", Checksum: "sha256:v1"},
	})

	// Different declared checksum → different cache key → fetch again.
	r2, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	res := r2.Resolve([]parse.DependencyRef{
		{ID: "hub-shared/dep-a", Version: "1.0.0", Checksum: "sha256:v2"},
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
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Each request encodes its own dep slug into the metadata.
		// Path: /api/shared/<slug>/<version>/metadata
		writeMetadata(w, "hub-shared/dep-"+r.URL.Path, "1.0.0", "sha256:x", []NodeInfo{
			{ID: "skill-from-" + r.URL.Path, Type: "skill"},
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
				{ID: fmt.Sprintf("hub-shared/dep-%d", i), Version: "1.0.0", Checksum: "sha256:x"},
			})
		}(i)
	}
	wg.Wait()

	// Cache file should be parseable JSON with entries from all goroutines.
	data, err := os.ReadFile(filepath.Join(cacheDir, "dep-nodes.json"))
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("cache file corrupted: %v\ncontents: %s", err, data)
	}
	if len(cf.Entries) != 8 {
		t.Errorf("expected 8 entries, got %d", len(cf.Entries))
	}
}

// Corrupt cache file must be treated as a miss, not fatal.
func TestResolve_CorruptCacheTreatedAsMiss(t *testing.T) {
	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, "dep-nodes.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadata(w, "hub-shared/dep-a", "1.0.0", "sha256:a", []NodeInfo{{ID: "skill-a", Type: "skill"}})
	})
	r, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	res := r.Resolve([]parse.DependencyRef{
		{ID: "hub-shared/dep-a", Version: "1.0.0", Checksum: "sha256:a"},
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
