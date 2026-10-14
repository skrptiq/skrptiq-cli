package scan

import (
	"fmt"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// GH#1058 — `research-paper-pipeline` was PUBLISHED and could not plan.
// `thesis-statement-crafter` ran first and read
// `{{steps.Literature Integration.output}}`, which runs second; the placeholder
// survived substitution and the GH#486 fail-fast halted the step:
//
//	step "Argument Construction" has unresolved variables:
//	{{steps.Literature Integration.output}}
//
// It produced nothing on TODAY's default, not only after the GH#941 derivation
// cutover — a customer installing it got a failure. `skrptiq order-diff`
// reported `status: cycle` for it, but order-diff is a tool somebody runs, not
// a gate that blocks, so nothing refused it at publication.
//
// THE DERIVATION IS NOT REIMPLEMENTED HERE, AND NEITHER IS ITS INPUT GRAPH.
// Both this clause and `order-diff` call `exec.ComputeOrderDiff` on a DB from
// `HydrateBundle`. Sharing the function alone was not enough, and the census
// proved it: computed against the SCAN's own temp DB — hydrated through the
// dep-aware bridge, which filters dep-provided edges and hydrates no dep nodes
// — `prd-generator-flow` reports a six-step cycle through `Language Polish`,
// a dependency-provided skill. `order-diff` reports `reordered` for the same
// package, and the engine runs it. One shared function over two different
// graphs is still a parallel authority, and its failure mode is the worst one
// available to a gate: refusing content that works. So the clause hydrates the
// package the way order-diff does and pays for a second temp DB to do it.
//
// `reordered` is DELIBERATELY NOT PART OF THIS. 23 published workflows reorder
// under derivation and 14 use positional `{{steps.previous.output}}`; those are
// the GH#1059 cutover question, a behaviour change that has not happened, and
// they want a different severity. A gate that refuses 23 packages today for
// that would be the wrong instrument.
//
// Census over the published catalogue before shipping (232 bundles, 91
// workflows): cycles 0, errors 0 — hub's figure, reproduced here through the
// scan's own hydration rather than taken on trust. The clause refuses a real
// defect class and rejects no existing content.
const (
	// CodePlanCycle fires when ordering derivation makes the step graph cyclic:
	// at least two steps disagree about which of them runs first, so no run
	// order exists and the workflow cannot plan at all.
	CodePlanCycle = "workflow.plan_cycle"

	// CodePlanUnbuildable fires when the plan cannot be built for any other
	// reason. Kept DISTINCT from the cycle code rather than merged into one
	// "plan status" finding: a cycle is a specific authoring mistake with a
	// named fix (reorder the two steps), while an unbuildable plan is an
	// unclassified failure whose message is all the author has to go on. The
	// missing-vs-invalid distinction this estate keeps relearning (K-034 D3).
	CodePlanUnbuildable = "workflow.plan_unbuildable"

	planStatusContractRef = "SKRPT-STRUCTURE-PRINCIPLES § Data-Flow & I/O Mapping"
)

// checkPlanStatus refuses a workflow whose derived plan status is cycle or
// error, computed on an order-diff-equivalent hydration of the package.
//
// A hydration failure is NOT reported here. The scan has already hydrated this
// package through its own bridge by the time this runs, and anything wrong with
// the package's shape has already been reported by that path with a better
// message; a second, differently-configured hydration failing is evidence about
// this clause's plumbing, not about the content.
func checkPlanStatus(pkg *parse.Package, absPath string) []ScanIssue {
	hb, err := HydrateBundle(absPath)
	if err != nil {
		return nil
	}
	defer hb.Close()

	var issues []ScanIssue
	for _, wf := range pkg.Nodes {
		if wf.Type != "workflow" {
			continue
		}
		od, err := exec.ComputeOrderDiff(hb.DB, wf.ID)
		if err != nil {
			issues = append(issues, ScanIssue{
				File:     relPath(absPath, wf.FilePath),
				NodeSlug: wf.ID,
				ValidationIssue: storage.ValidationIssue{
					Code:     CodePlanUnbuildable,
					Severity: storage.SeverityError,
					Message: fmt.Sprintf(
						"workflow %q cannot be planned, so it cannot run: %v",
						wf.ID, err),
					ContractRef:    planStatusContractRef,
					Field:          "execution",
					ReferencedSlug: wf.ID,
					ReferenceKind:  "workflow",
				},
			})
			continue
		}
		if od == nil || od.CycleError == "" {
			continue // ok or reordered — reordered is GH#1059, not this clause
		}
		// The engine's cycle error already NAMES the steps that disagree
		// ("… — A ↔ B", topo.go:76), so it is carried verbatim. Reporting
		// "status: cycle" alone would send the author to read the whole
		// execution block to find which two steps it meant.
		issues = append(issues, ScanIssue{
			File:     relPath(absPath, wf.FilePath),
			NodeSlug: wf.ID,
			ValidationIssue: storage.ValidationIssue{
				Code:     CodePlanCycle,
				Severity: storage.SeverityError,
				Message: fmt.Sprintf(
					"workflow %q has no possible run order, so it cannot plan: %s — reorder the execution block so each step runs after the step it reads, or correct the reference",
					wf.ID, od.CycleError),
				ContractRef:    planStatusContractRef,
				Field:          "execution",
				ReferencedSlug: wf.ID,
				ReferenceKind:  "workflow",
			},
		})
	}
	return issues
}
