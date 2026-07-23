package cli

import (
	"path/filepath"
	"testing"

	exec "github.com/skrptiq/engine/execution"
)

const orderDiffTestdata = "testdata/orderdiff"

// findDiff returns the diff row for a bundle by name.
func findDiff(t *testing.T, report orderDiffReport, bundle string) skrptDiff {
	t.Helper()
	for _, d := range report.Diffs {
		if d.Bundle == bundle {
			return d
		}
	}
	t.Fatalf("no diff row for bundle %q in report", bundle)
	return skrptDiff{}
}

// TestBuildOrderDiffReport is the end-to-end classification test over the three
// fixture bundles (one each of unchanged / reordered / cycle).
func TestBuildOrderDiffReport(t *testing.T) {
	report, err := buildOrderDiffReport(orderDiffTestdata)
	if err != nil {
		t.Fatalf("buildOrderDiffReport: %v", err)
	}

	if report.Bundles != 3 || report.Workflows != 3 {
		t.Errorf("want 3 bundles / 3 workflows, got %d / %d", report.Bundles, report.Workflows)
	}
	if report.Unchanged != 1 || report.Reordered != 1 || report.Cycles != 1 || report.Errors != 0 {
		t.Errorf("want unchanged=1 reordered=1 cycles=1 errors=0, got unchanged=%d reordered=%d cycles=%d errors=%d",
			report.Unchanged, report.Reordered, report.Cycles, report.Errors)
	}
	if report.PositionalHazards != 1 {
		t.Errorf("want 1 positional hazard, got %d", report.PositionalHazards)
	}
}

func TestBuildOrderDiffReport_Unchanged(t *testing.T) {
	report, err := buildOrderDiffReport(orderDiffTestdata)
	if err != nil {
		t.Fatal(err)
	}
	d := findDiff(t, report, "unchanged")
	if d.Status != "unchanged" {
		t.Errorf("status = %q, want unchanged", d.Status)
	}
	if len(d.DerivedEdges) != 0 || d.PositionalRefs {
		t.Errorf("unchanged row should carry no evidence, got edges=%v positional=%v", d.DerivedEdges, d.PositionalRefs)
	}
}

func TestBuildOrderDiffReport_Reordered(t *testing.T) {
	report, err := buildOrderDiffReport(orderDiffTestdata)
	if err != nil {
		t.Fatal(err)
	}
	d := findDiff(t, report, "reordered")
	if d.Status != "reordered" {
		t.Fatalf("status = %q, want reordered", d.Status)
	}
	// Authored order First → Second; derivation must invert it.
	if got := d.OldOrder; len(got) != 2 || got[0] != "First" || got[1] != "Second" {
		t.Errorf("oldOrder = %v, want [First Second]", got)
	}
	if got := d.NewOrder; len(got) != 2 || got[0] != "Second" || got[1] != "First" {
		t.Errorf("newOrder = %v, want [Second First]", got)
	}
	// Evidence: exactly the derived edge First ⇐ Second.
	if len(d.DerivedEdges) != 1 || d.DerivedEdges[0].Consumer != "First" || d.DerivedEdges[0].Producer != "Second" {
		t.Errorf("derivedEdges = %v, want [{First Second}]", d.DerivedEdges)
	}
	// The fixture also uses {{steps.previous.output}} — the hazard flag must fire.
	if !d.PositionalRefs {
		t.Error("positionalRefs = false, want true (fixture uses {{steps.previous.output}})")
	}
}

func TestBuildOrderDiffReport_Cycle(t *testing.T) {
	report, err := buildOrderDiffReport(orderDiffTestdata)
	if err != nil {
		t.Fatal(err)
	}
	d := findDiff(t, report, "cycle")
	if d.Status != "cycle" {
		t.Fatalf("status = %q, want cycle", d.Status)
	}
	if d.CycleError == "" {
		t.Error("cycle row must carry a CycleError message")
	}
}

// TestOrderDiffExitCode confirms the cycle (and error) path forces a non-zero
// exit, while a clean/reorders-only run would not.
func TestOrderDiffExitCode(t *testing.T) {
	report, err := buildOrderDiffReport(orderDiffTestdata)
	if err != nil {
		t.Fatal(err)
	}
	// Fixtures include a cycle → the command exits non-zero.
	if report.Cycles == 0 && report.Errors == 0 {
		t.Fatal("fixture set should contain a cycle")
	}
	failing := report.Cycles > 0 || report.Errors > 0
	if !failing {
		t.Error("expected a failing report (cycle present)")
	}
}

func TestFindBundles(t *testing.T) {
	bundles, err := findBundles(orderDiffTestdata)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 3 {
		t.Fatalf("want 3 bundles, got %d: %v", len(bundles), bundles)
	}
	// Sorted, and each is a directory containing skrptiq.yaml.
	for i, b := range bundles {
		if i > 0 && bundles[i-1] > b {
			t.Errorf("bundles not sorted: %v", bundles)
		}
		if filepath.Base(b) != "cycle" && filepath.Base(b) != "reordered" && filepath.Base(b) != "unchanged" {
			t.Errorf("unexpected bundle %q", b)
		}
	}
}

func TestFindBundles_Empty(t *testing.T) {
	// A directory with no skrptiq.yaml anywhere yields no bundles.
	bundles, err := findBundles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 0 {
		t.Errorf("want 0 bundles in empty dir, got %v", bundles)
	}
}

// so returns a StepOrder with a title == id, for terse rank tests.
func so(id string, pos int) exec.StepOrder {
	return exec.StepOrder{NodeID: id, NodeTitle: id, Position: pos}
}

func TestOrderChanged(t *testing.T) {
	cases := []struct {
		name     string
		old, new []exec.StepOrder
		want     bool
	}{
		{
			name: "identical sequential order",
			old:  []exec.StepOrder{so("a", 1), so("b", 2), so("c", 3)},
			new:  []exec.StepOrder{so("a", 1), so("b", 2), so("c", 3)},
			want: false,
		},
		{
			name: "same relative order, different position scale",
			old:  []exec.StepOrder{so("a", 0), so("b", 5), so("c", 9)},
			new:  []exec.StepOrder{so("a", 1), so("b", 2), so("c", 3)},
			want: false,
		},
		{
			name: "strict inversion",
			old:  []exec.StepOrder{so("a", 1), so("b", 2)},
			new:  []exec.StepOrder{so("b", 1), so("a", 2)},
			want: true,
		},
		{
			name: "sequential becomes parallel (still a change)",
			old:  []exec.StepOrder{so("a", 1), so("b", 2)},
			new:  []exec.StepOrder{so("a", 1), so("b", 1)},
			want: true,
		},
		{
			name: "parallel-tie linearised differently is NOT a change",
			// Both steps share a position in NEW; the flattened order is arbitrary
			// but the position rank is equal either way.
			old:  []exec.StepOrder{so("a", 1), so("b", 1)},
			new:  []exec.StepOrder{so("b", 1), so("a", 1)},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := orderChanged(tc.old, tc.new); got != tc.want {
				t.Errorf("orderChanged = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPositionRank(t *testing.T) {
	// Positions 0, 5, 9 dense-rank to 0, 1, 2.
	r := positionRank([]exec.StepOrder{so("a", 0), so("b", 5), so("c", 9)})
	if r["a"] != 0 || r["b"] != 1 || r["c"] != 2 {
		t.Errorf("ranks = %v, want a:0 b:1 c:2", r)
	}
	// Ties share a rank.
	r = positionRank([]exec.StepOrder{so("a", 2), so("b", 2), so("c", 7)})
	if r["a"] != 0 || r["b"] != 0 || r["c"] != 1 {
		t.Errorf("ranks = %v, want a:0 b:0 c:1", r)
	}
}

func TestSign(t *testing.T) {
	if sign(-4) != -1 || sign(0) != 0 || sign(9) != 1 {
		t.Errorf("sign broken: %d %d %d", sign(-4), sign(0), sign(9))
	}
}

func TestOrderTitles_Canonical(t *testing.T) {
	// Same-position steps render in title order regardless of input order, so
	// the report is deterministic despite arbitrary parallel-step linearisation.
	a := orderTitles([]exec.StepOrder{so("z", 1), so("a", 1), so("m", 2)})
	b := orderTitles([]exec.StepOrder{so("a", 1), so("z", 1), so("m", 2)})
	if len(a) != 3 || a[0] != "a" || a[1] != "z" || a[2] != "m" {
		t.Errorf("orderTitles = %v, want [a z m]", a)
	}
	if !equalStrings(a, b) {
		t.Errorf("orderTitles not canonical: %v vs %v", a, b)
	}
}

func TestUsesPositionalRef(t *testing.T) {
	if usesPositionalRef(exec.PlannedStep{Content: "read {{steps.previous.output}} now"}) != true {
		t.Error("positional ref in content not detected")
	}
	if usesPositionalRef(exec.PlannedStep{Context: map[string]string{"k": "{{ steps.previous.-1.output }}"}}) != true {
		t.Error("positional ref (with spaces / lookback) in context not detected")
	}
	if usesPositionalRef(exec.PlannedStep{Content: "read {{steps.Named Step.output}}"}) != false {
		t.Error("named ref must NOT count as a positional ref")
	}
	if usesPositionalRef(exec.PlannedStep{}) != false {
		t.Error("empty step must not report a positional ref")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
