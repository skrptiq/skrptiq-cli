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

// GH#1278 (GH#1267 step 4) — the consumer half of hub serving `inputs`.
//
// Hub's PR #213 added `declaredInputs()` to the shared-node projection. That
// change is INVISIBLE to anyone who has already resolved a dependency, because
// the cache key is id@version@checksum — three properties of the BUNDLE, none of
// which move when the SERVER changes what it sends. Exactly GH#1158's mechanism,
// one field over.
//
// Two things had to happen here, and the first is not in the issue title: the
// schema bump forces a refetch, but `NodeInfo` had no `Inputs` field, so the
// resolver would have re-fetched the declaration and discarded it on decode.
// A REFETCH IS NOT A RECEIPT. The bump alone would have been a one-line change
// that still delivered nothing, which is indistinguishable from the serving fix
// not working — the precise failure mode GH#1278 was filed to prevent.
//
// Verified against the live endpoint before writing any of this:
//   practice-question-generator@1.0.3 → inputs.question_count, inputs.exam_type
//   check-input-gaps@1.0.5, match-brand-voice@1.0.4, pii-masking@1.0.4,
//   segment-audience@1.0.5 → the `inputs` key is ABSENT, not an empty map
// so hub's "gated on declaration, an empty map is not a declaration" holds on
// the wire and the control below mirrors real served shapes rather than invented
// ones.

// TRANSPORT — the declaration arrives, and an unknown second-level key with it.
func TestGH1278_ServedInputsReachTheResolver(t *testing.T) {
	cacheDir := t.TempDir()
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadataRaw(w, uuidDepA, "1.0.0", "sha256:a", []map[string]any{{
			"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A",
			"inputs": map[string]any{
				"question_count": map[string]any{
					"label":    "Question Count",
					"required": true,
					"type":     "text",
					// A key no parser reads. It must still be here after the trip:
					// refusing it is the engine's job, and it cannot refuse what the
					// transport silently dropped.
					"requried": true,
				},
			},
		}})
	})
	r, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})

	// Asserted on the CACHE, because that is as far as cli's half goes:
	// storage.DepNodeSummary has no Inputs field, so there is no Result-side
	// surface to read. That hop is app's, and this test is honest about where
	// the data currently stops.
	n := soleCachedNode(t, cacheDir)
	if len(n.Inputs) == 0 {
		t.Fatalf("Inputs is empty — the declaration was decoded and discarded, which is " +
			"what the schema bump alone would NOT have fixed")
	}
	qc, ok := n.Inputs["question_count"].(map[string]any)
	if !ok {
		t.Fatalf("inputs.question_count did not survive as a map: %#v", n.Inputs["question_count"])
	}
	if qc["required"] != true {
		t.Errorf("required = %#v, want true", qc["required"])
	}
	if _, present := qc["requried"]; !present {
		t.Error("the UNKNOWN key was dropped in transport — a typed decode does exactly " +
			"this, and the key must survive in order to be refused by name")
	}
}

// The control the directive required: a projection that attached the field
// unconditionally would pass the test above perfectly. A dep serving no
// declaration must leave the field nil, not an empty map — an empty-but-present
// map is a declaration of nothing, which is not the same as no declaration.
func TestGH1278_NoInputsServed_FieldStaysNil(t *testing.T) {
	cacheDir := t.TempDir()
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeMetadataRaw(w, uuidDepA, "1.0.0", "sha256:a", []map[string]any{{
			"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A",
		}})
	})
	r, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})
	if n := soleCachedNode(t, cacheDir); n.Inputs != nil {
		t.Errorf("Inputs = %#v, want nil for a dep that served none", n.Inputs)
	}
}

// CACHE INVALIDATION — the claim the bump actually makes, and the one a constant
// cannot prove. A cache populated BEFORE the bump must be re-fetched AFTER it.
func TestGH1278_V3CacheSchemaDropped_AndRefetched(t *testing.T) {
	cacheDir := t.TempDir()
	// A well-formed v3 entry: valid under the previous schema, written by a
	// resolver that could not carry `inputs`, so it records their absence.
	v3 := []byte(`{
  "schemaVersion": 3,
  "entries": {
    "` + uuidDepA + `@1.0.0@sha256:a": {
      "id": "` + uuidDepA + `",
      "version": "1.0.0",
      "checksum": "sha256:a",
      "nodes": [{"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A"}],
      "fetched_at": "2026-11-16T00:00:00Z"
    }
  }
}`)
	if err := os.WriteFile(filepath.Join(cacheDir, "dep-nodes.json"), v3, 0o644); err != nil {
		t.Fatal(err)
	}

	var fetched int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetched, 1)
		writeMetadataRaw(w, uuidDepA, "1.0.0", "sha256:a", []map[string]any{{
			"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A",
			"inputs": map[string]any{
				"exam_type": map[string]any{"label": "Exam Type", "required": true, "type": "text"},
			},
		}})
	})
	r, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	r.Resolve([]parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	})

	if fetched != 1 {
		t.Fatalf("a schema-3 entry was SERVED (fetches = %d, want 1) — every machine that had "+
			"already scanned would keep reporting no declaration, because nothing in the "+
			"id@version@checksum key moves when the server changes what it sends. A "+
			"\"still doesn't fire\" report in that state is a cache hit, not a failure.", fetched)
	}
	if n := soleCachedNode(t, cacheDir); len(n.Inputs) == 0 {
		t.Fatalf("after the forced refetch the declaration is still absent: %#v", n)
	}
	// And the file is rewritten at the current version, so the next run is a
	// legitimate hit rather than a refetch loop.
	if v := cachedSchemaVersion(t, cacheDir); v != currentCacheSchemaVersion {
		t.Errorf("cache schema version = %d, want %d", v, currentCacheSchemaVersion)
	}
}

// The constant must actually have moved. This guards the failure mode of a bump
// that is reasoned about in a comment and never applied — the test above would
// still pass with the constant at 3 and the fixture saying 2.
func TestGH1278_CacheSchemaVersionIsAtLeast4(t *testing.T) {
	if currentCacheSchemaVersion < 4 {
		t.Fatalf("currentCacheSchemaVersion = %d; GH#1278 requires >= 4, or hub's served "+
			"`inputs` reaches nobody who has already resolved a dependency",
			currentCacheSchemaVersion)
	}
}

// The hit path is a different code path from the fetch path and has its own way
// of being wrong: the bump must not merely force one refetch and then lose the
// field again when the entry is read back.
func TestGH1278_InputsSurviveCacheRoundtrip(t *testing.T) {
	cacheDir := t.TempDir()
	var fetched int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetched, 1)
		writeMetadataRaw(w, uuidDepA, "1.0.0", "sha256:a", []map[string]any{{
			"id": "uuid-skill-a", "slug": "skill-a", "type": "skill", "title": "Skill A",
			"inputs": map[string]any{
				"exam_type": map[string]any{"label": "Exam Type", "required": true, "type": "text"},
			},
		}})
	})
	dep := []parse.DependencyRef{
		{ID: uuidDepA, Name: "dep-a", Version: "1.0.0", Checksum: "sha256:a"},
	}

	r1, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	r1.Resolve(dep) // populates the cache at v4

	r2, _ := New(Config{HubBaseURL: srv.URL, CacheDir: cacheDir})
	r2.Resolve(dep) // must be a HIT

	if fetched != 1 {
		t.Errorf("fetches = %d, want 1 — the second resolve should hit the cache, not refetch; "+
			"a bump that invalidates on every read is a different bug", fetched)
	}
	n := soleCachedNode(t, cacheDir)
	if len(n.Inputs) == 0 {
		t.Fatal("the declaration did not survive the cache round-trip")
	}
	if _, ok := n.Inputs["exam_type"].(map[string]any); !ok {
		t.Errorf("inputs.exam_type lost its shape on the hit path: %#v", n.Inputs["exam_type"])
	}
}

// --- helpers ---

func readCacheFile(t *testing.T, cacheDir string) cacheFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cacheDir, "dep-nodes.json"))
	if err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("cache file unparseable: %v", err)
	}
	return cf
}

func cachedSchemaVersion(t *testing.T, cacheDir string) int {
	t.Helper()
	return readCacheFile(t, cacheDir).SchemaVersion
}

// soleCachedNode returns the single cached node, failing if the cache does not
// hold exactly one entry with exactly one node — so an empty cache cannot be
// mistaken for a nil Inputs field.
func soleCachedNode(t *testing.T, cacheDir string) NodeInfo {
	t.Helper()
	cf := readCacheFile(t, cacheDir)
	if len(cf.Entries) != 1 {
		t.Fatalf("cache holds %d entries, want 1", len(cf.Entries))
	}
	for _, e := range cf.Entries {
		if len(e.Nodes) != 1 {
			t.Fatalf("cached entry holds %d nodes, want 1", len(e.Nodes))
		}
		return e.Nodes[0]
	}
	return NodeInfo{}
}
