package scan

import (
	"strings"
	"testing"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/storage"
)

// GH#1058 — `research-paper-pipeline` shipped and could not plan. These assert
// the floor: a cycle is refused naming the steps that disagree, and a correct
// workflow is accepted with its authored order intact.
//
// Census before shipping, through the scan's own path over the published
// catalogue (232 bundles): cycles 0, unbuildable 0 — hub's figure reproduced,
// so the clause refuses a real defect class and rejects no existing content.

// planIssues returns only this clause's findings.
func planIssues(t *testing.T, dir string) []ScanIssue {
	t.Helper()
	_, _, result := scanPkg(t, dir)
	var ours []ScanIssue
	for _, i := range result.Issues {
		if i.Code == CodePlanCycle || i.Code == CodePlanUnbuildable {
			ours = append(ours, i)
		}
	}
	return ours
}

// twoSkillPkg is a two-step workflow whose step bodies the caller supplies, so
// the data references between them are the only variable.
func twoSkillPkg(firstBody, secondBody string) map[string]string {
	return map[string]string{
		"workflows/wf.md": `---
type: workflow
id: wf
title: WF
description: "fixture"
connections:
  - target: first-skill
    type: uses
  - target: second-skill
    type: uses
execution:
  - skill: "first-skill"
    step_type: "generation"
    prompt: "first-prompt"
    output: { name: "first_out", type: "text" }
  - skill: "second-skill"
    step_type: "generation"
    prompt: "second-prompt"
    output: { name: "second_out", type: "text" }
---

## Overview

Fixture.
`,
		"skills/first-skill.md": `---
type: skill
id: first-skill
title: First Skill
description: "s"
---

First.
`,
		"skills/second-skill.md": `---
type: skill
id: second-skill
title: Second Skill
description: "s"
---

Second.
`,
		"prompts/first-prompt.md": `---
type: prompt
id: first-prompt
title: First Prompt
description: "p"
---

` + firstBody + `
`,
		"prompts/second-prompt.md": `---
type: prompt
id: second-prompt
title: Second Prompt
description: "p"
---

` + secondBody + `
`,
	}
}

// THE HEADLINE. Two steps that each read the other: no run order exists, so the
// workflow cannot plan, and the refusal must name both of them — "status:
// cycle" alone sends the author to read the whole execution block.
func TestGH1058_PlanCycle_IsRefusedNamingBothSteps(t *testing.T) {
	dir := writePkg(t, twoSkillPkg(
		"Continue from {{steps.Second Skill.output}}.",
		"Continue from {{steps.First Skill.output}}.",
	))

	issues := planIssues(t, dir)
	if len(issues) != 1 {
		t.Fatalf("want 1 cycle finding, got %+v", issues)
	}
	i := issues[0]
	if i.Code != CodePlanCycle {
		t.Errorf("code = %q, want %q", i.Code, CodePlanCycle)
	}
	if i.Severity != storage.SeverityError {
		t.Errorf("severity = %q, want error — a workflow that cannot plan must not publish", i.Severity)
	}
	for _, step := range []string{"First Skill", "Second Skill"} {
		if !strings.Contains(i.Message, step) {
			t.Errorf("message does not name %q, so the author cannot tell which steps disagree: %q", step, i.Message)
		}
	}

	// The floor, named: this gates.
	if _, code, _ := scanPkg(t, dir); code != 2 {
		t.Errorf("scan exit = %d, want 2", code)
	}
}

// THE OTHER HALF OF THE FLOOR: a correct workflow is accepted, and its authored
// order survives verbatim. A gate that refuses a cycle while quietly reordering
// everything else would be a worse problem than the one it fixes.
func TestGH1058_CorrectWorkflow_IsAcceptedAndItsOrderSurvives(t *testing.T) {
	dir := writePkg(t, twoSkillPkg(
		"Produce the material.",
		"Continue from {{steps.First Skill.output}}.",
	))

	if issues := planIssues(t, dir); len(issues) != 0 {
		t.Fatalf("a correct workflow was refused: %+v", issues)
	}
	if _, code, _ := scanPkg(t, dir); code != 0 {
		_, _, res := scanPkg(t, dir)
		t.Errorf("scan exit = %d, want 0; issues = %+v", code, res.Issues)
	}

	hb, err := HydrateBundle(dir)
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	defer hb.Close()
	od, err := exec.ComputeOrderDiff(hb.DB, "wf")
	if err != nil {
		t.Fatalf("order diff: %v", err)
	}
	if od.CycleError != "" {
		t.Fatalf("unexpected cycle: %s", od.CycleError)
	}
	var authored, derived []string
	for _, s := range od.Old {
		authored = append(authored, s.NodeTitle)
	}
	for _, s := range od.New {
		derived = append(derived, s.NodeTitle)
	}
	if strings.Join(authored, "|") != strings.Join(derived, "|") {
		t.Errorf("derivation changed the order of a correct workflow: authored %v, derived %v", authored, derived)
	}
	if strings.Join(authored, "|") != "First Skill|Second Skill" {
		t.Errorf("authored order = %v, want [First Skill Second Skill]", authored)
	}
}

// `reordered` is NOT this clause. 23 published workflows reorder under
// derivation; refusing them today for a cutover that has not happened (GH#1059)
// would refuse working content. Here the FIRST step reads the SECOND — one
// direction only, no cycle — which is precisely that class.
func TestGH1058_ReorderedWorkflow_IsNotRefused(t *testing.T) {
	dir := writePkg(t, twoSkillPkg(
		"Continue from {{steps.Second Skill.output}}.",
		"Produce the material.",
	))

	if issues := planIssues(t, dir); len(issues) != 0 {
		t.Errorf("a reordering workflow was refused by the cycle clause (that is GH#1059's question, not this one): %+v", issues)
	}
}
