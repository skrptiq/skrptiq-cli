package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/skrptiq-cli/internal/scan/depresolver"
)

// Synthetic UUIDs / checksums used by the test mocks.
const (
	testDepID       = "33333333-3333-3333-3333-cccccccccccc"
	testDepChecksum = "sha256:aaa"
	testSkillID     = "44444444-4444-4444-4444-dddddddddddd"
	testPromptID    = "55555555-5555-5555-5555-eeeeeeeeeeee"
)

// newMockHub returns a Hub server that serves one shared dep's
// metadata at /api/shared/<slug>/<version>/metadata with the K-037
// wire shape (id + slug + type + title + content).
func newMockHub(t *testing.T, slug, version string, nodes []map[string]any) *httptest.Server {
	t.Helper()
	wantPath := "/api/shared/" + slug + "/" + version + "/metadata"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       testDepID,
			"slug":     slug,
			"version":  version,
			"checksum": testDepChecksum,
			"nodes":    nodes,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// writeConsumerFixture writes a minimal consumer skrpt with the given
// dependencies block + a workflow that uses dep slugs. Returns the
// fixture dir.
func writeConsumerFixture(t *testing.T, deps string, workflowFrontmatter string) string {
	t.Helper()
	dir := t.TempDir()
	manifest := "name: test-consumer\nversion: 1.0.0\ndescription: GH#678 test\nauthor: test\n" + deps
	if err := os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	wf := "---\n" + workflowFrontmatter + "---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(dir, "workflows", "wf.md"), []byte(wf), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestMaterialiseSharedDeps_HappyPath — resolver returns 2 nodes
// (skill + prompt) and both land on disk as synthetic files with the
// expected frontmatter + body.
func TestMaterialiseSharedDeps_HappyPath(t *testing.T) {
	srv := newMockHub(t, "test-dep", "1.0.0", []map[string]any{
		{
			"id":      testSkillID,
			"slug":    "dep-skill",
			"type":    "skill",
			"title":   "Dep Skill",
			"content": "",
		},
		{
			"id":      testPromptID,
			"slug":    "dep-prompt",
			"type":    "prompt",
			"title":   "Dep Prompt",
			"content": "Process this: {{loop.item}}\n",
		},
	})

	deps := `dependencies:
  - id: "` + testDepID + `"
    name: test-dep
    version: "1.0.0"
    checksum: "` + testDepChecksum + `"
`
	dir := writeConsumerFixture(t, deps,
		"type: workflow\nid: wf\ntitle: \"WF\"\nmetadata:\n  execution:\n    - skill: dep-skill\n      prompt: dep-prompt\n      step_type: generation\n")

	r, err := depresolver.New(depresolver.Config{
		HubBaseURL: srv.URL,
		CacheDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("New resolver: %v", err)
	}

	if err := materialiseSharedDeps(dir, r); err != nil {
		t.Fatalf("materialiseSharedDeps: %v", err)
	}

	skillFile := filepath.Join(dir, "skills", "dep-skill.md")
	skillRaw, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("read synth skill: %v", err)
	}
	skill := string(skillRaw)
	for _, want := range []string{
		`id: "` + testSkillID + `"`,
		`type: "skill"`,
		`title: "Dep Skill"`,
		"(synthesised from hub-shared/dep-skill)",
		"(stripped for smoke corpus)",
	} {
		if !strings.Contains(skill, want) {
			t.Errorf("synth skill missing %q\n---\n%s\n---", want, skill)
		}
	}

	promptFile := filepath.Join(dir, "prompts", "dep-prompt.md")
	promptRaw, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("read synth prompt: %v", err)
	}
	prompt := string(promptRaw)
	for _, want := range []string{
		`id: "` + testPromptID + `"`,
		`type: "prompt"`,
		"Process this: {{loop.item}}",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("synth prompt missing %q\n---\n%s\n---", want, prompt)
		}
	}
}

// TestMaterialiseSharedDeps_LocalWinsOnCollision — when a local node
// file with the same slug exists, the synth path skips (mirrors
// engine ValidateWorkflowWithDeps local-wins semantics).
func TestMaterialiseSharedDeps_LocalWinsOnCollision(t *testing.T) {
	srv := newMockHub(t, "test-dep", "1.0.0", []map[string]any{
		{"id": testSkillID, "slug": "shared-skill", "type": "skill", "title": "Hub Title", "content": ""},
	})

	deps := `dependencies:
  - id: "` + testDepID + `"
    name: test-dep
    version: "1.0.0"
    checksum: "` + testDepChecksum + `"
`
	dir := writeConsumerFixture(t, deps,
		"type: workflow\nid: wf\ntitle: \"WF\"\n")

	// Pre-place a local skill at the same slug.
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	localBody := "---\ntype: skill\nid: local-skill-uuid\ntitle: \"Local Title\"\n---\nLocal body.\n"
	skillPath := filepath.Join(dir, "skills", "shared-skill.md")
	if err := os.WriteFile(skillPath, []byte(localBody), 0o644); err != nil {
		t.Fatal(err)
	}

	r, _ := depresolver.New(depresolver.Config{HubBaseURL: srv.URL, CacheDir: t.TempDir()})
	if err := materialiseSharedDeps(dir, r); err != nil {
		t.Fatalf("materialiseSharedDeps: %v", err)
	}

	got, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read post-synth: %v", err)
	}
	if !strings.Contains(string(got), "Local Title") {
		t.Errorf("local file should be untouched; got:\n%s", got)
	}
	if strings.Contains(string(got), "Hub Title") {
		t.Errorf("local-wins violated — synth overwrote local file:\n%s", got)
	}
}

// TestMaterialiseSharedDeps_EmptyDepsNoop — when the consumer has no
// dependencies block, the synth path is a no-op (no errors, no
// directories created).
func TestMaterialiseSharedDeps_EmptyDepsNoop(t *testing.T) {
	dir := writeConsumerFixture(t, "", "type: workflow\nid: wf\ntitle: \"WF\"\n")

	r, _ := depresolver.New(depresolver.Config{
		HubBaseURL: "http://nowhere.invalid",
		CacheDir:   t.TempDir(),
	})
	if err := materialiseSharedDeps(dir, r); err != nil {
		t.Fatalf("expected nil err on empty deps, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills")); !os.IsNotExist(err) {
		t.Errorf("expected no skills dir created, got err=%v", err)
	}
}

// TestMaterialiseSharedDeps_FetchFailureSurfaces — when the resolver
// can't reach Hub, the failure surfaces as an error (no silent
// half-resolution). Matches the plan's no-silent-degradation rule.
func TestMaterialiseSharedDeps_FetchFailureSurfaces(t *testing.T) {
	deps := `dependencies:
  - id: "` + testDepID + `"
    name: test-dep
    version: "1.0.0"
    checksum: "` + testDepChecksum + `"
`
	dir := writeConsumerFixture(t, deps, "type: workflow\nid: wf\ntitle: \"WF\"\n")

	r, _ := depresolver.New(depresolver.Config{
		HubBaseURL: "http://nowhere.invalid:9", // unreachable
		CacheDir:   t.TempDir(),
	})
	err := materialiseSharedDeps(dir, r)
	if err == nil {
		t.Fatal("expected fetch failure error, got nil")
	}
	if !strings.Contains(err.Error(), "fetch_failed") {
		t.Errorf("expected fetch_failed in error, got: %v", err)
	}
}

// TestGenerateFixture_WithSharedEndToEnd — full generateFixture
// invocation with --with-shared simulated. Source skrpt has a
// `dependencies:` block + a workflow referencing dep slugs; mocked
// Hub returns the dep nodes; output fixture parses cleanly via
// parse.ReadPackage.
func TestGenerateFixture_WithSharedEndToEnd(t *testing.T) {
	srv := newMockHub(t, "test-dep", "1.0.0", []map[string]any{
		{
			"id":      testSkillID,
			"slug":    "dep-skill",
			"type":    "skill",
			"title":   "Dep Skill",
			"content": "",
		},
		{
			"id":      testPromptID,
			"slug":    "dep-prompt",
			"type":    "prompt",
			"title":   "Dep Prompt",
			"content": "Process: {{steps.input.output}}",
		},
	})

	srcDir := t.TempDir()
	manifest := `name: end-to-end
version: 1.0.0
description: GH#678 end-to-end test
author: test
dependencies:
  - id: "` + testDepID + `"
    name: test-dep
    version: "1.0.0"
    checksum: "` + testDepChecksum + `"
`
	if err := os.WriteFile(filepath.Join(srcDir, "skrptiq.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	wf := `---
type: workflow
id: wf
title: "WF"
metadata:
  execution:
    - skill: dep-skill
      prompt: dep-prompt
      step_type: generation
---
Body.
`
	if err := os.WriteFile(filepath.Join(srcDir, "workflows", "wf.md"), []byte(wf), 0o644); err != nil {
		t.Fatal(err)
	}

	r, _ := depresolver.New(depresolver.Config{HubBaseURL: srv.URL, CacheDir: t.TempDir()})
	dstDir := t.TempDir()

	if err := generateFixture(srcDir, dstDir, r); err != nil {
		t.Fatalf("generateFixture: %v", err)
	}

	// Both synth nodes must exist on disk.
	for _, p := range []string{
		filepath.Join(dstDir, "skills", "dep-skill.md"),
		filepath.Join(dstDir, "prompts", "dep-prompt.md"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing synth file: %s", p)
		}
	}

	// And the whole fixture must parse cleanly via the engine reader
	// (the generateFixture validate step would already have failed
	// otherwise; this asserts the package count includes the synth
	// nodes).
	pkg, _, err := parse.ReadPackage(dstDir)
	if err != nil {
		t.Fatalf("post-gen parse: %v", err)
	}
	var sawSkill, sawPrompt bool
	for _, n := range pkg.Nodes {
		switch n.ID {
		case testSkillID:
			sawSkill = true
		case testPromptID:
			sawPrompt = true
		}
	}
	if !sawSkill || !sawPrompt {
		t.Errorf("expected both synth nodes in parsed Package; sawSkill=%v sawPrompt=%v", sawSkill, sawPrompt)
	}
}
