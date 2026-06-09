package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/hubapi"
	"github.com/skrptiq/engine/storage"
)

// Hub API drift-detection tests (GH#528). Opt-in via SKRPTIQ_HUB_URL.
// CI skips by default; run locally or in coordinated cross-repo
// verification to catch Hub wire-shape changes that haven't propagated
// to engine/hubapi types or depresolver.NodeInfo.
//
// Hub HTTP API has four type sources today; these tests pin two of them
// (engine/hubapi.Skrpt + depresolver.hubMetadata via the separate
// internal/scan/depresolver/live_test.go) against production response
// shapes. The other two sources (Hub server itself + electron-app's
// HubSkrpt / HubSharedMetadata) are tracked separately via GH#528 +
// GH#661.
//
// Failure modes these tests catch:
//   - Hub renames a field on its end without updating engine/hubapi.
//     JSON unmarshal silently zero-values the field; this surfaces as
//     Slug=="" / Name=="" on a populated Skrpt.
//   - Hub changes Skrpt → Skrpt-with-nested-shape, breaking parse
//     entirely (errors loud).
//   - Hub adds a required field the client doesn't tolerate as zero;
//     out of band today but worth a test if it bites.

func newLiveHubClient(t *testing.T) *hubapi.Client {
	t.Helper()
	hubURL := os.Getenv("SKRPTIQ_HUB_URL")
	if hubURL == "" {
		t.Skip("SKRPTIQ_HUB_URL not set — skipping live-Hub drift test")
	}
	dbPath := filepath.Join(t.TempDir(), "drift-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.SetSetting("HUB_BASE_URL", hubURL); err != nil {
		t.Fatalf("SetSetting HUB_BASE_URL: %v", err)
	}
	return hubapi.NewClient(db)
}

// isTransientNetworkError returns true for errors that mean Hub was
// unreachable rather than wire-shape-changed. Transient errors skip
// rather than fail so the test doesn't break CI on a Hub outage.
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "hub request failed") ||
		strings.Contains(msg, "hub 5") || // 5xx
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection refused")
}

// TestLiveHub_Browse_ParsesIntoSkrpt — GET /api/skrpts populates the
// hubapi.Skrpt fields the CLI's `hub list` / `hub search` commands
// surface. Slug + Name are the load-bearing keys; either being empty
// on a populated row means Hub changed the wire shape.
func TestLiveHub_Browse_ParsesIntoSkrpt(t *testing.T) {
	client := newLiveHubClient(t)
	skrpts, _, err := client.Browse("", "", 5, 0)
	if isTransientNetworkError(err) {
		t.Skipf("hub unreachable, skipping: %v", err)
	}
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	if len(skrpts) == 0 {
		t.Fatal("Browse returned 0 skrpts — Hub catalogue empty? Possible wire-shape drift " +
			"(envelope renamed → response unmarshalled into 0 entries)")
	}
	for i, s := range skrpts {
		if s.Slug == "" {
			t.Errorf("skrpts[%d].Slug is empty — Hub wire shape may have drifted from engine/hubapi.Skrpt", i)
		}
		if s.Name == "" {
			t.Errorf("skrpts[%d].Name is empty — same drift class", i)
		}
	}
}

// TestLiveHub_GetSkrpt_ParsesIntoSkrpt — GET /api/skrpts/<slug>. Uses
// Browse to discover a known-stable slug to fetch, so the test is
// self-bootstrapping (no pinned slug-UUID-checksum trio to maintain).
func TestLiveHub_GetSkrpt_ParsesIntoSkrpt(t *testing.T) {
	client := newLiveHubClient(t)
	skrpts, _, err := client.Browse("", "", 1, 0)
	if isTransientNetworkError(err) {
		t.Skipf("hub unreachable: %v", err)
	}
	if err != nil {
		t.Fatalf("Browse bootstrap: %v", err)
	}
	if len(skrpts) == 0 {
		t.Fatal("no skrpts available to test GetSkrpt against")
	}
	slug := skrpts[0].Slug
	if slug == "" {
		t.Fatal("bootstrap Skrpt.Slug is empty — Browse already drifted")
	}

	detail, err := client.GetSkrpt(slug)
	if isTransientNetworkError(err) {
		t.Skipf("hub unreachable: %v", err)
	}
	if err != nil {
		t.Fatalf("GetSkrpt(%q): %v", slug, err)
	}
	if detail == nil {
		t.Fatalf("GetSkrpt(%q) returned nil with no error — wire shape drift?", slug)
	}
	if detail.Slug != slug {
		t.Errorf("GetSkrpt(%q).Slug = %q; want %q", slug, detail.Slug, slug)
	}
}
