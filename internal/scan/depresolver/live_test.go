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
func TestResolve_LiveHub_SampledBundle(t *testing.T) {
	hubURL := os.Getenv("SKRPTIQ_HUB_URL")
	if hubURL == "" {
		t.Skip("SKRPTIQ_HUB_URL not set — skipping live-Hub integration test")
	}

	r, err := New(Config{HubBaseURL: hubURL, CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const sampledChecksum = "sha256:9940a8d84c3240b3dcea7fe8b88939ac43898ea4138c706ca17d783e7894bb23"
	res := r.Resolve([]parse.DependencyRef{
		{ID: "hub-shared/llm-service", Version: "1.0.0", Checksum: sampledChecksum},
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
	if dep != "hub-shared/llm-service" {
		t.Errorf("ProvidedSlugs[llm-service] = %q; want hub-shared/llm-service", dep)
	}
}
