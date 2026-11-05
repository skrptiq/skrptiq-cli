package scan

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GH#1158 — the resolver must CARRY `context_params`, not merely decode it.
//
// Three layers of one defect: hub's endpoint serves the declaration, the engine
// enforces its shape, and between them this repo's resolver dropped the field on
// the floor. The enforcement had been merged and live since 2026-11-01 with ZERO
// reachable subjects, because a shared dependency's declaration never arrived.
//
// So the floor here is REACH, not green. The rule being exercised
// (`workflow.context_param_shape_mismatch`) already worked for a package-local
// node and had never once worked through a dependency — which is exactly why a
// test that declares `supplied_by` on a local skill would prove nothing. In every
// test below the declaration exists ONLY on the dep-served node.

// mockHubMetadataAny is mockHubMetadata with `[]map[string]any` nodes, so a node
// can carry a nested `context_params` object. The original takes
// `[]map[string]string` and cannot express one; it keeps its signature because
// thirteen call sites depend on it.
func mockHubMetadataAny(t *testing.T, id, slug, version, checksum string, nodes []map[string]any) http.Handler {
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

// sharedSkillNodes returns the dep's node list: one skill carrying the given
// context_params block, plus nothing else. `contextParams` is passed through
// verbatim so a test can serve a shape the Go types do not model.
func sharedSkillNodes(contextParams map[string]any) []map[string]any {
	n := map[string]any{
		"id":    "44444444-4444-4444-4444-dddddddddddd",
		"slug":  "shared-skill",
		"type":  "skill",
		"title": "Shared Skill",
	}
	if contextParams != nil {
		n["context_params"] = contextParams
	}
	return []map[string]any{n}
}

// FLOOR 1 — the headline, and the only one that distinguishes this change from
// no change at all: a `supplied_by` declaration on a SHARED dependency reaches
// the validator and refuses a from_step binding against a supplied_by:input slot.
func TestGH1158_DepDeclaredSuppliedBy_RefusesStepBinding(t *testing.T) {
	h := mockHubMetadataAny(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa",
		sharedSkillNodes(map[string]any{
			"tone": map[string]any{"supplied_by": "input", "required": true},
		}))

	res := runScanWithDeps(t, "testdata/gh1158-dep-supplied-by", h, Options{})

	if !hasIssueCode(res, "workflow.context_param_shape_mismatch") {
		t.Fatalf("a supplied_by:input slot declared on a DEP-PROVIDED skill did not refuse a "+
			"from_step binding — the declaration is not reaching the validator, which is the "+
			"whole of GH#1158; issues = %s", issueCodes(res))
	}

	// The message must name the declaring site, or an author cannot find the
	// contract to change: the slot is declared in a dependency, not in this package.
	var msg string
	for _, i := range res.Issues {
		if i.Code == "workflow.context_param_shape_mismatch" {
			msg = i.Message
		}
	}
	if !strings.Contains(msg, "shared-skill") {
		t.Errorf("refusal does not name the declaring dep node, so it points nowhere: %q", msg)
	}
}

// The negative half of floor 1. Without this, floor 1 passes just as well against
// a validator that refuses every binding it sees.
func TestGH1158_DepDeclaredSuppliedByInput_AcceptsInputBinding(t *testing.T) {
	h := mockHubMetadataAny(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa",
		sharedSkillNodes(map[string]any{
			"tone": map[string]any{"supplied_by": "input", "required": true},
		}))

	res := runScanWithDeps(t, "testdata/gh1158-dep-supplied-by-ok", h, Options{})

	if hasIssueCode(res, "workflow.context_param_shape_mismatch") {
		t.Errorf("a from_input binding against a supplied_by:input slot was refused — "+
			"the rule is firing on the binding's existence rather than its shape; issues = %s",
			issueCodes(res))
	}
	if res.ErrorCount != 0 {
		t.Errorf("errors = %d, want 0 for a conforming consumer: %s", res.ErrorCount, issueCodes(res))
	}
}

// FLOOR 4 — the population floor. A resolver that carried NOTHING would pass
// every refusal test above by never reaching them for a different reason, so
// assert the declarations are actually received and non-empty. This is the test
// that fails loudly if `ContextParams` is dropped from NodeInfo again.
func TestGH1158_DeclarationsActuallyArrive_PopulationFloor(t *testing.T) {
	h := mockHubMetadataAny(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa",
		sharedSkillNodes(map[string]any{
			"tone": map[string]any{"supplied_by": "input", "required": true},
		}))

	res := runScanWithDeps(t, "testdata/gh1158-dep-supplied-by", h, Options{})

	// The dep resolved at all (no unresolved-slug noise standing in for success).
	for _, i := range res.Issues {
		if i.Code == "dependency.unresolved_slug" || i.Code == "dependency.fetch_failed" {
			t.Fatalf("the dep did not resolve, so no refusal below is evidence of anything: %s", i.Message)
		}
	}
	// And the refusal that can only come from a received declaration did fire.
	if !hasIssueCode(res, "workflow.context_param_shape_mismatch") {
		t.Fatalf("zero declarations reached the validator — population floor is zero; issues = %s",
			issueCodes(res))
	}
}

// NOTE on the unknown-key rule (GH#1139 `validateContextParamKeys`): it is NOT
// asserted here, and the omission is deliberate rather than an oversight. That
// rule is invoked from `ValidateNodeShape` — the PER-NODE path — while a dep's
// declaration only ever exists on the synthetic node the engine builds inside
// `ValidateWorkflowWithDeps`. `ValidateNodeShape` never sees that synthetic, so
// the rule has no reach through a dependency no matter what this resolver
// carries. Asserting it here would either fail for a reason outside this repo or
// tempt a second key-check in the CLI, which is the parallel-authority mistake
// GH#1058 was about. The transport half — that an unknown key SURVIVES the trip
// rather than being dropped — is what this repo owns, and it is asserted at the
// resolver in depresolver_test.go. The reach gap is surfaced on GH#1158.

// A dep that serves NO context_params must behave exactly as before — the field
// is additive, and `omitempty` plus a nil map must not become an empty
// declaration that the validator reads as "declares nothing in particular".
func TestGH1158_DepWithoutContextParams_IsUnchanged(t *testing.T) {
	h := mockHubMetadataAny(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa",
		sharedSkillNodes(nil))

	res := runScanWithDeps(t, "testdata/gh1158-dep-supplied-by-ok", h, Options{})

	if hasIssueCode(res, "workflow.context_param_shape_mismatch") {
		t.Errorf("a dep serving no declaration produced a shape refusal: %s", issueCodes(res))
	}
	if hasIssueCode(res, "workflow.context_param_supplied_by_invalid") {
		t.Errorf("a dep serving no declaration produced a misspelling refusal: %s", issueCodes(res))
	}
}

// issueCodes renders the codes for a failure message. A bare count tells the next
// reader nothing about which rule fired.
func issueCodes(r ScanResult) string {
	if len(r.Issues) == 0 {
		return "(none)"
	}
	seen := make([]string, 0, len(r.Issues))
	for _, i := range r.Issues {
		seen = append(seen, i.Code)
	}
	return strings.Join(seen, ", ")
}

// readCacheFile / writeCacheFile helpers for the schema-version floor live with
// the resolver's own tests; this file keeps to the scan-level reach assertions.
var _ = os.ReadFile
var _ = filepath.Join

// skillAndPromptNodes serves BOTH declaration sites from the dep — the shape the
// live catalogue actually has, where `audience-segmentation` (skill) and
// `segment-audience` (prompt) are separate shared packages and each declares the
// same `source` slot.
func skillAndPromptNodes(skillCP, promptCP map[string]any) []map[string]any {
	skill := map[string]any{
		"id": "44444444-4444-4444-4444-dddddddddddd", "slug": "shared-skill",
		"type": "skill", "title": "Shared Skill",
	}
	if skillCP != nil {
		skill["context_params"] = skillCP
	}
	prompt := map[string]any{
		"id": "55555555-5555-5555-5555-eeeeeeeeeeee", "slug": "shared-prompt",
		"type": "prompt", "title": "Shared Prompt",
		"content": "Write it in this tone: {{step.context.tone}}",
	}
	if promptCP != nil {
		prompt["context_params"] = promptCP
	}
	return []map[string]any{skill, prompt}
}

// TWO DEP-PROVIDED SITES THAT AGREE — passes.
//
// This is the state the catalogue has to be in for the chain to be complete, and
// the test exists to say what "aligned" means concretely: the same slot declared
// at both sites with the same `supplied_by` and `required`.
func TestGH1158_BothSitesDepProvided_Agreeing_Passes(t *testing.T) {
	decl := map[string]any{
		"tone": map[string]any{"supplied_by": "input", "required": true},
	}
	h := mockHubMetadataAny(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa",
		skillAndPromptNodes(decl, decl))

	res := runScanWithDeps(t, "testdata/gh1158-dep-skill-and-prompt", h, Options{})

	if hasIssueCode(res, "workflow.context_param_declared_twice") {
		t.Errorf("two AGREEING declarations were refused as a disagreement: %s", issueCodes(res))
	}
	if res.ErrorCount != 0 {
		t.Errorf("errors = %d, want 0 when both sites agree: %s", res.ErrorCount, issueCodes(res))
	}
}

// TWO DEP-PROVIDED SITES THAT DISAGREE — refused, and this is the live condition.
//
// MEASURED on the catalogue with this change in place: 11 refusals across 10
// consumer packages, every one of them this exact shape — the shared SKILL
// declares `supplied_by: input` (GH#1144) and the paired shared PROMPT declares
// the same slot with no `supplied_by` at all, so the merge sees "input" against
// "". The disagreement is not new; it has been in the content since GH#1144 and
// was invisible because NEITHER site's declaration reached the validator. This
// change makes both arrive, and `declared_twice` then does its job.
//
// The test is written in the direction of the defect deliberately: it asserts the
// refusal, so if hub aligns the prompt-side declarations and someone later
// reverts that alignment, this still fails rather than passing quietly.
func TestGH1158_BothSitesDepProvided_Disagreeing_IsRefused(t *testing.T) {
	h := mockHubMetadataAny(t, testDepUUID, "test-dep", "1.0.0", "sha256:aaa",
		skillAndPromptNodes(
			map[string]any{"tone": map[string]any{"supplied_by": "input", "required": true}},
			// The live prompt shape: label/description/required, no supplied_by.
			map[string]any{"tone": map[string]any{"required": true, "label": "Tone"}},
		))

	res := runScanWithDeps(t, "testdata/gh1158-dep-skill-and-prompt", h, Options{})

	if !hasIssueCode(res, "workflow.context_param_declared_twice") {
		t.Fatalf("a skill declaring supplied_by:input against a prompt declaring the same slot "+
			"with no supplied_by was NOT refused — this is the condition measured on 10 live "+
			"consumers, and it must not resolve silently; issues = %s", issueCodes(res))
	}
}
