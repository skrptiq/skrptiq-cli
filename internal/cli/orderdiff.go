package cli

// GH#941 — catalogue order-diff harness (the hard gate before the engine's
// ordering-derivation flips on).
//
// For every published skrpt, the engine builds its plan twice — data-ref
// ordering-derivation OFF (today's positional behaviour) then ON — and reports
// whether the step order changes and why. The report is the artefact a human
// reviews before `DeriveOrderingEdges` goes default-on: we NEVER silently
// reorder the catalogue. Each workflow lands in one of three buckets:
//
//   - unchanged  — order identical either way (safe to flip).
//   - reordered  — order differs; the row carries the before/after step
//                  sequence AND the derived data-ref edges that forced the
//                  change, so a human can adjudicate more-correct vs regression.
//   - cycle      — derivation makes the graph cyclic: a latent authoring bug,
//                  publish-blocking per #282, and a hard non-zero exit here.
//
// Deliberately NO mechanical "improvement" heuristic: adjudicating
// strictly-more-correct vs regression is the whole point of the gate — a
// heuristic could quietly pass a regression. This surfaces the evidence only.

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/storage"
	"github.com/skrptiq/skrptiq-cli/internal/scan"
)

// positionalRefPattern matches {{steps.previous(.−N).output}} — the POSITIONAL
// lookback derivation deliberately does NOT self-order (derive_edges.go). A
// reordered skrpt that also carries one of these is a hazard: the derivation's
// named edges can float an un-named producer past a step that relied on
// positional adjacency, silently breaking the positional link (the #927 class).
var positionalRefPattern = regexp.MustCompile(`\{\{\s*steps\.previous`)

// edgeRef is one derived data-ref ordering edge, consumer ⇐ producer, by title.
type edgeRef struct {
	Consumer string `json:"consumer"` // the step that reads {{steps.X.output}} / from_step
	Producer string `json:"producer"` // the step X it now must follow
}

// skrptDiff is the per-workflow order-diff row.
type skrptDiff struct {
	Bundle       string    `json:"bundle"`
	Workflow     string    `json:"workflow"`
	WorkflowID   string    `json:"workflowId"`
	Status       string    `json:"status"` // unchanged | reordered | cycle | error
	OldOrder     []string  `json:"oldOrder,omitempty"`
	NewOrder     []string  `json:"newOrder,omitempty"`
	DerivedEdges []edgeRef `json:"derivedEdges,omitempty"`
	// PositionalRefs flags a reorder whose steps also use {{steps.previous.output}}.
	// Such a skrpt cannot be certified more-correct on the derived edges alone —
	// the reorder may have broken a positional link (the #927 hazard class).
	PositionalRefs bool   `json:"positionalRefs,omitempty"`
	CycleError     string `json:"cycleError,omitempty"`
	Error          string `json:"error,omitempty"` // non-cycle build/hydrate failure
}

// orderDiffReport is the whole-catalogue report.
type orderDiffReport struct {
	Root      string `json:"root"`
	Bundles   int    `json:"bundles"`
	Workflows int    `json:"workflows"`
	Unchanged int    `json:"unchanged"`
	Reordered int    `json:"reordered"`
	Cycles    int    `json:"cycles"`
	Errors    int    `json:"errors"`
	// PositionalHazards counts reordered skrpts that also use {{steps.previous}}
	// — the subset most likely to hide a regression behind a reorder.
	PositionalHazards int         `json:"positionalHazards"`
	Diffs             []skrptDiff `json:"diffs"`
}

// OrderDiff runs the catalogue order-diff over every skrpt bundle under a
// directory and prints the blast-radius report. Exit: 0 clean (no cycles, no
// errors — reorders alone do NOT fail, they await human review); 1 if any
// cycle or hard error; 2 on bad arguments.
func OrderDiff(args []string) int {
	fs := flag.NewFlagSet("order-diff", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Emit the report as structured JSON")
	fs.Parse(args)

	root := fs.Arg(0)
	if root == "" {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq order-diff [--json] <catalogue-dir>")
		fmt.Fprintln(os.Stderr, "  Runs the engine's ordering-derivation diff over every skrpt bundle under <catalogue-dir>.")
		return ExitBadArgs
	}

	report, err := buildOrderDiffReport(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitBadArgs
	}

	if *jsonOut {
		outputJSON(os.Stdout, report)
	} else {
		writeOrderDiffTable(os.Stdout, report)
	}

	if report.Cycles > 0 || report.Errors > 0 {
		return ExitFailed
	}
	return ExitOK
}

// buildOrderDiffReport hydrates every bundle under root and classifies each of
// its workflows. Returns an error only for a bad root (walk failure / no
// bundles); per-bundle hydrate failures are recorded as error rows, not
// returned, so one broken bundle never aborts the catalogue run.
func buildOrderDiffReport(root string) (orderDiffReport, error) {
	bundles, err := findBundles(root)
	if err != nil {
		return orderDiffReport{}, fmt.Errorf("walking %s: %w", root, err)
	}
	if len(bundles) == 0 {
		return orderDiffReport{}, fmt.Errorf("no skrpt bundles (directories with skrptiq.yaml) found under %s", root)
	}

	absRoot, _ := filepath.Abs(root)
	report := orderDiffReport{Root: absRoot, Bundles: len(bundles)}

	for _, bundle := range bundles {
		rel := bundleLabel(absRoot, bundle)
		hb, hErr := scan.HydrateBundle(bundle)
		if hErr != nil {
			report.Errors++
			report.Diffs = append(report.Diffs, skrptDiff{Bundle: rel, Status: "error", Error: hErr.Error()})
			continue
		}
		wfs := hb.Workflows
		sort.Slice(wfs, func(i, j int) bool { return wfs[i].Title < wfs[j].Title })
		for _, wf := range wfs {
			report.Workflows++
			report.Diffs = append(report.Diffs, diffWorkflow(hb.DB, rel, wf, &report))
		}
		hb.Close()
	}
	return report, nil
}

// diffWorkflow classifies one workflow and, for a reorder, attaches the derived
// data-ref edges that forced it.
func diffWorkflow(db *storage.DB, bundle string, wf scan.WorkflowRef, report *orderDiffReport) skrptDiff {
	d := skrptDiff{Bundle: bundle, Workflow: wf.Title, WorkflowID: wf.ID}

	od, err := exec.ComputeOrderDiff(db, wf.ID)
	if err != nil {
		report.Errors++
		d.Status = "error"
		d.Error = err.Error()
		return d
	}

	switch {
	case od.CycleError != "":
		report.Cycles++
		d.Status = "cycle"
		d.CycleError = od.CycleError
		d.OldOrder = orderTitles(od.Old)
	case orderChanged(od.Old, od.New):
		report.Reordered++
		d.Status = "reordered"
		d.OldOrder = orderTitles(od.Old)
		d.NewOrder = orderTitles(od.New)
		d.DerivedEdges, d.PositionalRefs = analyseReorder(db, wf.ID)
		if d.PositionalRefs {
			report.PositionalHazards++
		}
	default:
		report.Unchanged++
		d.Status = "unchanged"
	}
	return d
}

// orderChanged reports whether the derivation-ON ordering changes the relative
// order of any step pair vs derivation-OFF. It is the deterministic,
// parallel-safe replacement for OrderDiff.Changed: that field compares the
// FLATTENED step sequence, which linearises same-position (parallel) steps in
// arbitrary map-iteration order, so it flaps run-to-run and false-flags a mere
// parallelisation. Here we compare by dense position rank, pairwise — OLD and
// NEW use different position scales (authored vs topological), so only relative
// order is meaningful. A pair whose strict before/after (or parallel) relation
// differs between OLD and NEW is a real reorder.
func orderChanged(oldOrder, newOrder []exec.StepOrder) bool {
	ro, rn := positionRank(oldOrder), positionRank(newOrder)
	ids := make([]string, 0, len(ro))
	for id := range ro {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			a, b := ids[i], ids[j]
			if sign(ro[a]-ro[b]) != sign(rn[a]-rn[b]) {
				return true
			}
		}
	}
	return false
}

// positionRank maps each step's NodeID to its dense rank (0-based tier index by
// ascending Position). Deterministic: depends only on the stable positions, not
// on the arbitrary order of parallel steps within a tier.
func positionRank(order []exec.StepOrder) map[string]int {
	posSet := map[int]bool{}
	for _, s := range order {
		posSet[s.Position] = true
	}
	positions := make([]int, 0, len(posSet))
	for p := range posSet {
		positions = append(positions, p)
	}
	sort.Ints(positions)
	rankOf := make(map[int]int, len(positions))
	for i, p := range positions {
		rankOf[p] = i
	}
	r := make(map[string]int, len(order))
	for _, s := range order {
		r[s.NodeID] = rankOf[s.Position]
	}
	return r
}

func sign(x int) int {
	switch {
	case x < 0:
		return -1
	case x > 0:
		return 1
	default:
		return 0
	}
}

// analyseReorder rebuilds the derivation-ON plan and returns (a) the synthetic
// data-ref ordering edges (consumer ⇐ producer, by step title) that forced the
// change, and (b) whether any step also uses positional {{steps.previous}} —
// the hazard flag. Best-effort: a build failure yields no edges (the reorder is
// still reported; ComputeOrderDiff already succeeded on this plan).
func analyseReorder(db *storage.DB, workflowID string) ([]edgeRef, bool) {
	prev := exec.DeriveOrderingEdges
	exec.DeriveOrderingEdges = true
	plan, err := exec.BuildExecutionPlan(db, workflowID, nil)
	exec.DeriveOrderingEdges = prev
	if err != nil || plan == nil {
		return nil, false
	}

	var steps []exec.PlannedStep
	title := map[string]string{}
	positional := false
	record := func(s exec.PlannedStep) {
		steps = append(steps, s)
		title[s.NodeID] = s.NodeTitle
		if usesPositionalRef(s) {
			positional = true
		}
	}
	for _, g := range plan.PositionGroups {
		for _, s := range g.Steps {
			record(s)
		}
		if g.LoopBlock != nil {
			for _, s := range g.LoopBlock.Body {
				record(s)
			}
			if g.LoopBlock.VerifierStep != nil {
				record(*g.LoopBlock.VerifierStep)
			}
		}
	}

	var out []edgeRef
	for _, e := range exec.DeriveDataDependencyEdges(steps) {
		out = append(out, edgeRef{
			Consumer: titleOr(title, e.SourceID),
			Producer: titleOr(title, e.TargetID),
		})
	}
	// Canonical order: the derived-edge slice follows step order, which is
	// arbitrary within parallel tiers — sort for a stable report.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Consumer != out[j].Consumer {
			return out[i].Consumer < out[j].Consumer
		}
		return out[i].Producer < out[j].Producer
	})
	return out, positional
}

// usesPositionalRef reports whether a step reads {{steps.previous.output}} in
// its prompt body or any context value — the positional link derivation cannot
// preserve across a reorder.
func usesPositionalRef(s exec.PlannedStep) bool {
	if positionalRefPattern.MatchString(s.Content) {
		return true
	}
	for _, v := range s.Context {
		if positionalRefPattern.MatchString(v) {
			return true
		}
	}
	return false
}

// findBundles walks root and returns every directory that contains a
// skrptiq.yaml (a skrpt bundle), sorted. Bundles do not nest, so it does not
// descend into a bundle once found — a container dir like examples/_shared/
// (no manifest of its own) is descended into and its sub-bundles collected.
func findBundles(root string) ([]string, error) {
	var bundles []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(p, "skrptiq.yaml")); statErr == nil {
			bundles = append(bundles, p)
			return filepath.SkipDir
		}
		return nil
	})
	sort.Strings(bundles)
	return bundles, err
}

// bundleLabel is the bundle path relative to the catalogue root (falls back to
// the base name), for compact display.
func bundleLabel(absRoot, bundle string) string {
	if rel, err := filepath.Rel(absRoot, bundle); err == nil && rel != "." {
		return rel
	}
	return filepath.Base(bundle)
}

// orderTitles renders an ordering as titles in a CANONICAL sequence — sorted by
// (Position, NodeTitle) — so same-position (parallel) steps, whose flattened
// order is arbitrary, render deterministically. Steps sharing a position are
// parallel; the derived edges show what actually constrains order.
func orderTitles(order []exec.StepOrder) []string {
	sorted := make([]exec.StepOrder, len(order))
	copy(sorted, order)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Position != sorted[j].Position {
			return sorted[i].Position < sorted[j].Position
		}
		return sorted[i].NodeTitle < sorted[j].NodeTitle
	})
	out := make([]string, 0, len(sorted))
	for _, s := range sorted {
		out = append(out, s.NodeTitle)
	}
	return out
}

func titleOr(title map[string]string, id string) string {
	if t := title[id]; t != "" {
		return t
	}
	return id
}

// writeOrderDiffTable renders the human-readable report.
func writeOrderDiffTable(w io.Writer, r orderDiffReport) {
	fmt.Fprintf(w, "Catalogue order-diff — %s\n", r.Root)
	fmt.Fprintf(w, "%d bundles, %d workflows scanned\n\n", r.Bundles, r.Workflows)
	fmt.Fprintf(w, "  unchanged  %d\n", r.Unchanged)
	fmt.Fprintf(w, "  reordered  %d  (of which positional-ref hazards: %d)\n", r.Reordered, r.PositionalHazards)
	fmt.Fprintf(w, "  cycle      %d\n", r.Cycles)
	fmt.Fprintf(w, "  error      %d\n", r.Errors)

	// Reordered — the set needing human adjudication.
	if r.Reordered > 0 {
		fmt.Fprintf(w, "\nREORDERED — human review required (is the new order strictly more correct?):\n")
		for _, d := range r.Diffs {
			if d.Status != "reordered" {
				continue
			}
			hazard := ""
			if d.PositionalRefs {
				hazard = "  ⚠ POSITIONAL-REF HAZARD"
			}
			fmt.Fprintf(w, "\n  %s › %s%s\n", d.Bundle, d.Workflow, hazard)
			fmt.Fprintf(w, "    old: %s\n", joinArrow(d.OldOrder))
			fmt.Fprintf(w, "    new: %s\n", joinArrow(d.NewOrder))
			if len(d.DerivedEdges) > 0 {
				fmt.Fprintf(w, "    derived edges (consumer ⇐ producer):\n")
				for _, e := range d.DerivedEdges {
					fmt.Fprintf(w, "      %s ⇐ %s\n", e.Consumer, e.Producer)
				}
			}
			if d.PositionalRefs {
				fmt.Fprintf(w, "    ⚠ also uses {{steps.previous.output}} — the reorder may have broken a positional link; NOT certifiable on derived edges alone.\n")
			}
		}
	}

	// Cycles — publish-blocking (#282).
	if r.Cycles > 0 {
		fmt.Fprintf(w, "\nCYCLE — publish-blocking (#282):\n")
		for _, d := range r.Diffs {
			if d.Status != "cycle" {
				continue
			}
			fmt.Fprintf(w, "\n  %s › %s\n", d.Bundle, d.Workflow)
			fmt.Fprintf(w, "    %s\n", d.CycleError)
		}
	}

	// Errors — could not diff; block a clean certification.
	if r.Errors > 0 {
		fmt.Fprintf(w, "\nERROR — could not diff:\n")
		for _, d := range r.Diffs {
			if d.Status != "error" {
				continue
			}
			label := d.Bundle
			if d.Workflow != "" {
				label = d.Bundle + " › " + d.Workflow
			}
			fmt.Fprintf(w, "  %s: %s\n", label, d.Error)
		}
	}

	fmt.Fprintln(w)
	switch {
	case r.Cycles > 0 || r.Errors > 0:
		fmt.Fprintln(w, "NOT CLEAN — resolve cycles/errors before flipping derivation on.")
	case r.Reordered > 0:
		fmt.Fprintf(w, "%d reorder(s) await human adjudication before flipping derivation on.\n", r.Reordered)
	default:
		fmt.Fprintln(w, "CLEAN — no reorders, cycles, or errors.")
	}
}

func joinArrow(titles []string) string {
	out := ""
	for i, t := range titles {
		if i > 0 {
			out += " → "
		}
		out += t
	}
	if out == "" {
		return "(no steps)"
	}
	return out
}
