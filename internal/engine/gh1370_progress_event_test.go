package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// GH#1370 — three renderers switch on `exec.ProgressEvent.Type` and none had a
// `default`, so an event type this build could not render produced NOTHING: no
// line, no placeholder, no indication something happened and was dropped. The
// engine gaining a type made the renderer silently behind it.
//
// `cli/destinations.go:84` already handled the same family WITH a default and
// argued for it — *"an unhandled pending result is the silence this whole change
// exists to prevent, and a new type must not inherit it."* Three sites predated
// that convention.

// Direction 1 — an unrecognised type produces a line that NAMES it. The planted
// value cannot occur naturally: the engine's vocabulary is `execution-started`,
// `step-*`, and a fabricated one is the only way to tell "the default fired" from
// "the data happened to contain it".
func TestGH1370_UnknownProgressEventLine_NamesThePlantedType(t *testing.T) {
	const planted = "quantum-entangled"

	line := UnknownProgressEventLine(planted)

	if !strings.Contains(line, planted) {
		t.Errorf("the line does not name the event type: %q", line)
	}
	for _, want := range []string{"not one this build knows how to render", "nothing was", "unaffected"} {
		if !strings.Contains(line, want) {
			t.Errorf("the line is missing %q: %q", want, line)
		}
	}
	// It must not read as a failure. No verdict depends on this, and a run must
	// not look failed because the renderer is behind the engine.
	for _, mustNot := range []string{"failed", "error", "refus"} {
		if strings.Contains(strings.ToLower(line), mustNot) {
			t.Errorf("the line reads as a failure (%q): %q", mustNot, line)
		}
	}
}

// ONE PHRASING, THREE RENDERERS. The structural half: every switch over
// `evt.Type` must have a `default`, and that default must go through this one
// function rather than spelling the sentence again. A fourth renderer added
// without one fails here.
//
// Direction 2 — THE CONTROL, and the one that matters: the known event types must
// still be handled as CASES, never swept into the default. A default that fires
// on a known type adds a line to every run, which is how it gets deleted.
func TestGH1370_EveryProgressEventSwitch_HasADefaultAndKeepsItsCases(t *testing.T) {
	root := repoRoot(t)

	// The case labels each site handled before GH#1370, recorded so that moving a
	// known type into the default is a test failure rather than a silent change.
	expected := map[string][]string{
		"internal/cli/run.go": {
			"execution-started", "step-started", "step-chunk", "step-completed",
			"step-failed", "step-rejected", "step-awaiting-input",
		},
		"internal/app/app.go": {
			"execution-started", "step-started", "step-chunk", "step-completed",
			"step-failed", "step-awaiting-input",
		},
	}

	found := map[string]int{}
	for rel := range expected {
		path := filepath.Join(root, rel)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}

		seen := map[string]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok || !isEventTypeSwitch(sw) {
				return true
			}
			found[rel]++

			hasDefault, callsHelper := false, false
			for _, stmt := range sw.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				if cc.List == nil {
					hasDefault = true
					callsHelper = strings.Contains(nodeText(t, fset, path, cc), "UnknownProgressEventLine")
					continue
				}
				for _, e := range cc.List {
					if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if v, err := strconv.Unquote(lit.Value); err == nil {
							seen[v] = true
						}
					}
				}
			}

			pos := fset.Position(sw.Pos())
			if !hasDefault {
				t.Errorf("%s:%d switches on evt.Type with no default — an unknown event type "+
					"renders nothing, which is the silence GH#1370 is about", rel, pos.Line)
			} else if !callsHelper {
				t.Errorf("%s:%d has a default that does not call UnknownProgressEventLine — "+
					"one phrasing for one decision; a second spelling is the drift this retires",
					rel, pos.Line)
			}
			return true
		})

		for _, want := range expected[rel] {
			if !seen[want] {
				t.Errorf("%s no longer handles %q as a case — a known type swept into the "+
					"default would add a line to every run", rel, want)
			}
		}
	}

	// Both files must still contain the switches; a rename that loses them would
	// otherwise pass this test vacuously.
	for rel := range expected {
		if found[rel] == 0 {
			t.Errorf("%s: no evt.Type switch found at all — this guard graded nothing", rel)
		}
	}
	if found["internal/app/app.go"] < 2 {
		t.Errorf("internal/app/app.go: found %d evt.Type switches, want 2 (the run and the resume)",
			found["internal/app/app.go"])
	}
}

// isEventTypeSwitch reports whether a switch's subject is `evt.Type`.
func isEventTypeSwitch(sw *ast.SwitchStmt) bool {
	sel, ok := sw.Tag.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "Type" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == "evt"
}

func nodeText(t *testing.T, fset *token.FileSet, path string, n ast.Node) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	start := fset.Position(n.Pos()).Offset
	end := fset.Position(n.End()).Offset
	if start < 0 || end > len(src) || start > end {
		return ""
	}
	return string(src[start:end])
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return root
}
