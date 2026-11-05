package depresolver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/skrptiq/engine/parse"
)

// GH#1158 — the resolver's half of a three-layer defect. Hub's endpoint serves
// `context_params`, the engine enforces its shape, and in between this resolver
// decoded the field and discarded it, so the enforcement had no reachable
// subject from the day it merged.
//
// Two things are asserted here and nowhere else:
//
//  1. TRANSPORT. An unknown second-level key survives the trip. This is the
//     reason the field is `map[string]any` and not a typed shape — a typed
//     decode drops what it does not model, and the key would be gone before any
//     gate could name it. The transport must not be what decides what counts as
//     unknown.
//  2. CACHE INVALIDATION. The cache key is id@version@checksum, all three
//     properties of the BUNDLE — none of which move when the SERVER changes
//     what it sends. A schema-2 entry therefore records the absence of a
//     declaration that now exists, and must be dropped rather than served.

// writeMetadataRaw serves a nodes array built from raw maps, so a node can carry
// a `context_params` block — including keys no Go type models. writeMetadata
// takes []NodeInfo and cannot express an unmodelled key, which is precisely the
// thing worth testing.
func writeMetadataRaw(w http.ResponseWriter, id, version, checksum string, nodes []map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":       id,
		"version":  version,
		"checksum": checksum,
		"nodes":    nodes,
	})
}

// TRANSPORT — the declaration arrives on DepNodeSummary, keys and all.
func TestGH1158_ContextParamsReachDepNodeSummary(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadataRaw(w, uuidDepA, "1.0.0", "sha256:a", []map[string]any{{
			"id":    "uuid-skill-a",
			"slug":  "skill-a",
			"type":  "skill",
			"title": "Skill A",
			"context_params": map[string]any{
				"tone": map[string]any{
					"supplied_by": "input",
					"required":    true,
					// A key no parser reads. It must still be HERE after the trip;
					// refusing it is the engine's job, but it cannot refuse what the
					// transport silently dropped.
					"sopplied_by": "input",
				},
			},
		}})
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})

	if len(res.DepNodes) != 1 {
		t.Fatalf("DepNodes = %d, want 1: %+v", len(res.DepNodes), res.DepNodes)
	}
	cp := res.DepNodes[0].ContextParams
	if len(cp) == 0 {
		t.Fatalf("ContextParams is empty — the declaration was decoded and discarded, " +
			"which is the whole of GH#1158")
	}
	tone, ok := cp["tone"].(map[string]any)
	if !ok {
		t.Fatalf("context_params.tone did not survive as a map: %#v", cp["tone"])
	}
	if tone["supplied_by"] != "input" {
		t.Errorf("supplied_by = %#v, want \"input\"", tone["supplied_by"])
	}
	if _, present := tone["sopplied_by"]; !present {
		t.Error("the UNKNOWN key was dropped in transport — a typed decode would do exactly " +
			"this, and the key must survive in order to be refused by name")
	}
}

// A dep that serves no declaration must leave the field nil rather than an empty
// map: the engine gates on `len(dep.ContextParams) > 0`, and an empty-but-present
// map is a declaration of nothing, which is not the same as no declaration.
func TestGH1158_NoContextParamsServed_FieldStaysNil(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadataRaw(w, uuidDepA, "1.0.0", "sha256:a", []map[string]any{{
			"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A",
		}})
	})
	r := newTestResolver(t, srv.URL)
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	if len(res.DepNodes) != 1 {
		t.Fatalf("DepNodes = %d, want 1", len(res.DepNodes))
	}
	if res.DepNodes[0].ContextParams != nil {
		t.Errorf("ContextParams = %#v, want nil for a dep that served none",
			res.DepNodes[0].ContextParams)
	}
}

// CACHE INVALIDATION — a schema-2 file is DISCARDED, not read.
//
// This is the floor that stops the change being invisible in the field. CI starts
// with a cold cache and would pass without the bump; a developer machine that has
// already scanned would keep serving a pre-GH#1158 entry under an unchanged
// id@version@checksum key until natural expiry. Observed, not theorised: hub's
// cache held `has_context_params=False` for four objects after the endpoint began
// serving them.
func TestGH1158_V2CacheSchemaDropped_AndRefetched(t *testing.T) {
	cacheDir := t.TempDir()
	// A well-formed v2 entry: valid under the previous schema, and written by a
	// resolver that could not carry context_params, so it records their absence.
	v2 := []byte(`{
  "schemaVersion": 2,
  "entries": {
    "` + uuidDepA + `@1.0.0@sha256:a": {
      "id": "` + uuidDepA + `",
      "version": "1.0.0",
      "checksum": "sha256:a",
      "nodes": [{"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A"}],
      "fetched_at": "2026-11-01T00:00:00Z"
    }
  }
}`)
	if err := os.WriteFile(filepath.Join(cacheDir, "dep-nodes.json"), v2, 0o644); err != nil {
		t.Fatal(err)
	}

	var fetched int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetched, 1)
		writeMetadataRaw(w, uuidDepA, "1.0.0", "sha256:a", []map[string]any{{
			"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A",
			"context_params": map[string]any{
				"tone": map[string]any{"supplied_by": "input", "required": true},
			},
		}})
	})
	r, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	res := r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})

	if fetched != 1 {
		t.Fatalf("a schema-2 entry was SERVED (fetches = %d, want 1) — every machine that had "+
			"already scanned would keep reporting no declaration, because nothing in the "+
			"id@version@checksum key moves when the server changes what it sends", fetched)
	}
	if len(res.DepNodes) != 1 || len(res.DepNodes[0].ContextParams) == 0 {
		t.Fatalf("after the forced refetch the declaration is still absent: %+v", res.DepNodes)
	}

	// Rewritten at the current version, so the next run is a legitimate hit.
	data, err := os.ReadFile(filepath.Join(cacheDir, "dep-nodes.json"))
	if err != nil {
		t.Fatalf("cache file missing after fresh fetch: %v", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("cache file unparseable: %v", err)
	}
	if cf.SchemaVersion != currentCacheSchemaVersion {
		t.Errorf("cache schema version = %d, want %d", cf.SchemaVersion, currentCacheSchemaVersion)
	}
}

// The constant must actually have moved. A bump that is reasoned about in a
// comment and not applied is the failure mode this guards: the test above would
// still pass if `currentCacheSchemaVersion` were 2 and the fixture said 1.
func TestGH1158_CacheSchemaVersionIsAtLeast3(t *testing.T) {
	if currentCacheSchemaVersion < 3 {
		t.Fatalf("currentCacheSchemaVersion = %d; GH#1158 requires >= 3, or every "+
			"already-scanned machine keeps serving a pre-declaration cache entry",
			currentCacheSchemaVersion)
	}
}

// A cache entry written at the CURRENT schema round-trips the declaration, so the
// bump does not merely force one refetch and then lose the field again on the hit
// path. The hit path is a different code path from the fetch path and has its own
// way of being wrong.
func TestGH1158_ContextParamsSurviveCacheRoundtrip(t *testing.T) {
	cacheDir := t.TempDir()
	var fetched int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetched, 1)
		writeMetadataRaw(w, uuidDepA, "1.0.0", "sha256:a", []map[string]any{{
			"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A",
			"context_params": map[string]any{
				"tone": map[string]any{"supplied_by": "input", "required": true},
			},
		}})
	})
	dep := []parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	}

	r1, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	r1.Resolve(dep) // populates the cache

	r2, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	res := r2.Resolve(dep) // must be a HIT

	if fetched != 1 {
		t.Errorf("second resolve refetched (fetches = %d, want 1) — not exercising the hit path", fetched)
	}
	if len(res.DepNodes) != 1 {
		t.Fatalf("DepNodes = %d, want 1", len(res.DepNodes))
	}
	tone, ok := res.DepNodes[0].ContextParams["tone"].(map[string]any)
	if !ok {
		t.Fatalf("declaration lost on the CACHE HIT path: %+v", res.DepNodes[0].ContextParams)
	}
	if tone["supplied_by"] != "input" {
		t.Errorf("supplied_by after round-trip = %#v, want \"input\"", tone["supplied_by"])
	}
}
