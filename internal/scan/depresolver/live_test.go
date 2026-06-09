package depresolver

import (
	"os"
	"testing"

	"github.com/skrptiq/engine/parse"
)

// TestResolve_LiveHub_SampledBundle is the K-035 sanity beat against
// the live Hub catalogue. Opt-in via SKRPTIQ_HUB_URL — CI skips by
// default; Hub agent (and Ben, locally) sets the env var to exercise
// the full HTTP + parse + checksum-verify + cache-write chain against
// production.
//
// The mocked-Hub suite (depresolver_test.go) exhaustively covers
// failure modes. This test exists solely to prove the wire contract
// (Option A endpoint shape, GH#630) actually holds in production.
//
// Pinned bundle: llm-service@1.0.0. Hub sampled this bundle at
// GH#630 comment 2026-05-31 20:38Z and published the checksum below.
// The bundle is immutable per Cache-Control: max-age=31536000, immutable
// on the metadata endpoint — checksum drift here means Hub broke its
// immutability contract, not a transient flake.
//
// Per K-037 (UUID canonical identity) the manifest's id: is now a UUID
// rather than the legacy `hub-shared/<slug>` form. Catalogue UUID for
// llm-service@1.0.0 captured from the same sampled-verification
// response.
func TestResolve_LiveHub_SampledBundle(t *testing.T) {
	hubURL := os.Getenv("SKRPTIQ_HUB_URL")
	if hubURL == "" {
		t.Skip("SKRPTIQ_HUB_URL not set — skipping live-Hub integration test")
	}

	r, err := New(Config{HubBaseURL: hubURL, CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const (
		sampledUUID     = "fcab9536-a13e-49c9-b0c3-bb3097682e69"
		sampledChecksum = "sha256:9940a8d84c3240b3dcea7fe8b88939ac43898ea4138c706ca17d783e7894bb23"
	)
	res := r.Resolve([]parse.DependencyRef{
		{ID: sampledUUID, Name: "llm-service", Version: "1.0.0", Checksum: sampledChecksum},
	})

	// Transient unreachability (DNS, network drop, CF outage) →
	// skip, don't fail. Distinguishes "Hub down" from "contract broken".
	for _, i := range res.Issues {
		if i.Code == "dependency.fetch_failed" {
			t.Skipf("Hub unreachable, skipping: %s", i.Message)
		}
	}

	if len(res.Issues) != 0 {
		t.Fatalf("live Hub resolution returned issues: %+v", res.Issues)
	}
	dep, hit := res.ProvidedSlugs["llm-service"]
	if !hit {
		t.Fatalf("expected llm-service in ProvidedSlugs; got %+v", res.ProvidedSlugs)
	}
	// Values are the dep's logical slug (Name) for human readability.
	if dep != "llm-service" {
		t.Errorf("ProvidedSlugs[llm-service] = %q; want llm-service", dep)
	}
}

// TestResolve_LiveHub_DepNodesHaveNonEmptySlug — GH#528 / GH#650 / GH#654
// drift guard. The `/api/shared/<slug>/<v>/metadata` wire shape must
// populate `nodes[].slug` (post-GH#650). If Hub silently drops the
// field, NodeInfo.Slug deserialises to "" and the engine's
// depNodes-merge skips the entry (engine guard: `if dep.Slug == ""`),
// surfacing as false-positive `dependency.unresolved_slug` at scan
// time with no other signal. This test pins that wire-shape
// expectation explicitly so wire-drift fails loudly here instead of
// silently breaking real consumer scans.
func TestResolve_LiveHub_DepNodesHaveNonEmptySlug(t *testing.T) {
	hubURL := os.Getenv("SKRPTIQ_HUB_URL")
	if hubURL == "" {
		t.Skip("SKRPTIQ_HUB_URL not set — skipping live-Hub drift test")
	}
	r, err := New(Config{HubBaseURL: hubURL, CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const (
		sampledUUID     = "fcab9536-a13e-49c9-b0c3-bb3097682e69"
		sampledChecksum = "sha256:9940a8d84c3240b3dcea7fe8b88939ac43898ea4138c706ca17d783e7894bb23"
	)
	res := r.Resolve([]parse.DependencyRef{
		{ID: sampledUUID, Name: "llm-service", Version: "1.0.0", Checksum: sampledChecksum},
	})
	for _, i := range res.Issues {
		if i.Code == "dependency.fetch_failed" {
			t.Skipf("Hub unreachable: %s", i.Message)
		}
	}
	if len(res.DepNodes) == 0 {
		t.Fatalf("expected at least one DepNode; got %d", len(res.DepNodes))
	}
	for i, n := range res.DepNodes {
		if n.Slug == "" {
			t.Errorf("DepNodes[%d].Slug is empty — Hub wire shape did not populate `nodes[].slug` "+
				"(GH#650 extension regression). NodeInfo: %+v", i, n)
		}
		if n.ID == "" {
			t.Errorf("DepNodes[%d].ID is empty — Hub wire shape did not populate `nodes[].id`", i)
		}
	}
}
