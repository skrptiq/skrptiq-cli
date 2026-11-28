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

	// THE ENGINE'S WHOLE PROGRESS VOCABULARY, measured from the emitters on
	// 2026-11-28 — `execution/runner.go`, `execution/resume.go`, `execution/step.go`
	// and `execution/reject.go` — not from what the renderers happened to handle.
	//
	// THAT DISTINCTION IS THE FIX TO THIS TEST'S FIRST DRAFT, which recorded only
	// the cases each site already had and therefore PASSED ON THE BROKEN STATE BY
	// CONSTRUCTION. Five real lifecycle types were handled nowhere, all five arrive
	// on the same `onProgress` these sites pass, and the new default would have
	// printed "this build does not know how to render it" on every successful run.
	// A control derived from the thing it grades is not a control.
	required := []string{
		"execution-started", "execution-completed", "execution-failed",
		"execution-paused", "execution-cancelled", "execution-resumed",
		"step-started", "step-chunk", "step-completed", "step-failed",
		"step-awaiting-input",
	}
	// `step-rejected` is emitted by `execution/reject.go`, and `cli/run.go` is the
	// ONLY caller of `RejectGate` in this repo, so the TUI's renderers cannot
	// receive it. Allowed everywhere, required nowhere — the one legitimate
	// difference between the three switches.

	// EVERY evt.Type SWITCH IS GRADED ON ITS OWN, with no list of sites to keep in
	// step. The first draft accumulated labels per FILE, so a type handled in one of
	// `app.go`'s two renderers satisfied the assertion for both — "both switches are
	// found" is not "both switches agree". The second keyed on `file:line`, which a
	// single edit above a switch invalidated. This asks the same question of
	// whatever it finds, so a NEW renderer is held to the same bar automatically
	// rather than being silently ungraded.
	switches := 0
	for _, rel := range []string{"internal/cli/run.go", "internal/app/app.go"} {
		path := filepath.Join(root, rel)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok || !isEventTypeSwitch(sw) {
				return true
			}
			switches++
			where := rel + ":" + strconv.Itoa(fset.Position(sw.Pos()).Line)

			seen := map[string]bool{}
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

			if !hasDefault {
				t.Errorf("%s switches on evt.Type with no default — an unknown event type "+
					"renders nothing, which is the silence GH#1370 is about", where)
			} else if !callsHelper {
				t.Errorf("%s has a default that does not call UnknownProgressEventLine — one "+
					"phrasing for one decision; a second spelling is the drift this retires", where)
			}

			for _, w := range required {
				if !seen[w] {
					t.Errorf("%s does not handle %q as a case — it is in the engine's progress "+
						"vocabulary, so reaching the default would add a line to every run", where, w)
				}
			}
			return true
		})
	}

	// Three renderers today: one in `cli/run.go`, two in `app/app.go`. A guard that
	// found none would pass silently, which is the shape this whole issue is about.
	if switches != 3 {
		t.Errorf("found %d evt.Type switches, want 3 — if a renderer was added, it is now "+
			"graded by this test and the count needs updating deliberately; if one was lost, "+
			"this guard was about to grade nothing", switches)
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
