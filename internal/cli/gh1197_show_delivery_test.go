package cli

import (
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1197 (response delivery E3) — `skrptiq show run` renders the engine's headline and element
// list, and decides inline-versus-references from the TERMINAL HEIGHT.
//
// The rule under test, as ruled: a parts-only split that fits the panel prints its CONTENT; a
// length split, or anything that does not fit, prints REFERENCES. A pipe is not a terminal and
// carries everything.
//
// WHY THE VIEWPORT IS INJECTED rather than read inside the renderer: the acceptance row is a
// statement about an 80x25 terminal, and a test that could only observe the real one would be
// untestable in CI and would silently assert nothing. `renderDelivery` takes the viewport so the
// panel size is a parameter of the measurement.
//
// Both directions are asserted throughout, per the GH#1215 amendment: a renderer that always
// printed references would pass a "does it list elements" test perfectly.

func el(id, title, content string, lines int) storage.RunElement {
	return storage.RunElement{
		ID: id, Title: title, Content: content, Lines: lines, Bytes: len(content), Status: "completed",
	}
}

func deliv(class storage.ResponseClass, reason string, els ...storage.RunElement) *storage.RunDelivery {
	return &storage.RunDelivery{
		Classification: storage.RunClassification{Class: class, Reason: reason},
		Headline:       "completed: 2 of 2 steps completed",
		Elements:       els,
	}
}

func render(t *testing.T, d *storage.RunDelivery, vp viewport) string {
	t.Helper()
	var b strings.Builder
	if code := renderDelivery(&b, "1784401202433405000", d, vp); code != ExitOK {
		t.Fatalf("renderDelivery exit = %d, want %d", code, ExitOK)
	}
	return b.String()
}

// THE ACCEPTANCE ROW. A 178-byte two-step run on an 80x25 terminal prints its content, not two ids.
// This is the row that proves the programme fixed what it set out to.
func TestGH1197_ShortMultiPartRunOn80x25_PrintsContentNotIDs(t *testing.T) {
	d := deliv(storage.ClassHeadlineElements, "2 parts",
		el("run:a::0", "Compose", `{"composed": true}`, 1),
		el("run:b::0", "Send", `{"sent": true}`, 1),
	)
	out := render(t, d, viewport{isTerminal: true, height: 25})

	if !strings.Contains(out, `{"composed": true}`) || !strings.Contains(out, `{"sent": true}`) {
		t.Errorf("content missing — a short two-part run must print what it produced:\n%s", out)
	}
	if strings.Contains(out, "run:a::0") || strings.Contains(out, "run:b::0") {
		t.Errorf("element ids were printed for a run that fits the panel — this is the wall-of-ids "+
			"failure the issue exists to fix:\n%s", out)
	}
}

// The other direction: the same parts-only reason, but the bodies do not fit. References, with
// sizes, and the full content one command away.
func TestGH1197_MultiPartRunThatDoesNotFit_PrintsReferences(t *testing.T) {
	big := strings.Repeat("line\n", 80)
	d := deliv(storage.ClassHeadlineElements, "2 parts",
		el("run:a::0", "Analysis", big, 80),
		el("run:b::0", "Summary", big, 80),
	)
	out := render(t, d, viewport{isTerminal: true, height: 25})

	if !strings.Contains(out, "run:a::0") || !strings.Contains(out, "run:b::0") {
		t.Errorf("expected element ids when the bodies cannot fit:\n%s", out)
	}
	if strings.Contains(out, big) {
		t.Error("printed a full body that cannot fit the panel — that is the wall of text")
	}
	if !strings.Contains(out, "show run 17844012 <element-id>") {
		t.Errorf("no way offered to retrieve one element in full:\n%s", out)
	}
}

// A LENGTH split prints references even when it would fit, because the engine's reason says the
// content itself is long — the viewport is not the only consideration, it is the tie-breaker for
// a parts-only split.
func TestGH1197_LengthSplitPrintsReferencesEvenWhenItFits(t *testing.T) {
	d := deliv(storage.ClassHeadlineElements, "26 lines is over 25",
		el("run:a::0", "Report", "short body\n", 1),
	)
	out := render(t, d, viewport{isTerminal: true, height: 25})
	if !strings.Contains(out, "run:a::0") {
		t.Errorf("a length split must print references, not bodies:\n%s", out)
	}
	if !strings.Contains(out, "26 lines is over 25") {
		t.Errorf("the engine's own reason should be shown rather than paraphrased:\n%s", out)
	}
}

// Plain and short text is fine as it is (Ben, 14 November): no headline, no list header, no ids.
func TestGH1197_InlineRunPrintsTheTextAlone(t *testing.T) {
	d := deliv(storage.ClassInline, "short, single-part text",
		el("run:a::0", "Answer", "Thirty-seven.", 1))
	out := render(t, d, viewport{isTerminal: true, height: 25})

	if strings.TrimSpace(out) != "Thirty-seven." {
		t.Errorf("a short single-part answer must be just the answer, got:\n%q", out)
	}
}

// Zero elements is the ABSENCE of a plain-text case, not an instance of one. Printing nothing would
// read as success with an empty answer.
func TestGH1197_NoElementsSaysSoRatherThanPrintingNothing(t *testing.T) {
	d := deliv(storage.ClassInline, "short, single-part text")
	out := render(t, d, viewport{isTerminal: true, height: 25})

	if !strings.Contains(out, "no output") {
		t.Errorf("a run that produced nothing must say so:\n%s", out)
	}
	if strings.Contains(out, "0 elements") {
		t.Error("printed an empty-list scaffold — the no-element path must not read as an error")
	}
}

// A pipe is a console-style caller by definition: everything, unchanged, regardless of size.
func TestGH1197_NotATerminalCarriesEverything(t *testing.T) {
	big := strings.Repeat("line\n", 200)
	d := deliv(storage.ClassHeadlineElements, "2 parts",
		el("run:a::0", "Analysis", big, 200),
		el("run:b::0", "Summary", "tail\n", 1),
	)
	out := render(t, d, viewport{}) // not a terminal

	if !strings.Contains(out, big) || !strings.Contains(out, "tail") {
		t.Error("a pipe must carry the full content of every element")
	}
	if strings.Contains(out, "<element-id>") {
		t.Error("offered a follow-up command to a pipe, which cannot run one")
	}
}

// A terminal that will not report its height must not be treated as infinite: guessing large prints
// the wall. 24 rows is the conservative assumption.
func TestGH1197_UnknownTerminalHeightDoesNotAssumeInfinite(t *testing.T) {
	big := strings.Repeat("line\n", 100)
	d := deliv(storage.ClassHeadlineElements, "2 parts",
		el("run:a::0", "A", big, 100), el("run:b::0", "B", big, 100))
	if bodiesFit(d.Elements, 24) {
		t.Error("200 lines of body reported as fitting 24 rows")
	}
}

// Parallel loop members share a node and therefore a title. Four identically-labelled rows are
// unreadable, so repeated titles are numbered — and unrepeated ones are left alone rather than
// given a pointless "(1/1)".
func TestGH1197_RepeatedTitlesAreDisambiguated(t *testing.T) {
	labels := elementLabels([]storage.RunElement{
		{Title: "Competitor Profiling"}, {Title: "Competitor Profiling"},
		{Title: "Competitor Profiling"}, {Title: "Competitor Profiling"},
		{Title: "Language Polish"},
	})
	want := []string{
		"Competitor Profiling (1/4)", "Competitor Profiling (2/4)",
		"Competitor Profiling (3/4)", "Competitor Profiling (4/4)",
		"Language Polish",
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("label[%d] = %q, want %q", i, labels[i], want[i])
		}
	}
}

// The loud degrade. `Fallback` is on the payload rather than in a log precisely so it can be shown,
// and it must appear BEFORE the content — a reader who scrolls past it has not been told.
func TestGH1197_FallbackIsAnnouncedBeforeTheContent(t *testing.T) {
	d := deliv(storage.ClassInline, "fell back to inline", el("run:f::0", "full output", "everything", 1))
	d.Classification.Fallback = true
	d.Classification.FallbackReason = "classification unavailable"
	out := render(t, d, viewport{isTerminal: true, height: 25})

	if !strings.Contains(out, "classification unavailable") {
		t.Errorf("the fallback was not announced:\n%s", out)
	}
	if strings.Index(out, "classification unavailable") > strings.Index(out, "everything") {
		t.Error("the fallback notice printed after the content")
	}
}

// One element in full, never truncated: "full content is always retrievable" is the contract.
func TestGH1197_RenderElementPrintsOneInFullAndRefusesAnUnknownID(t *testing.T) {
	body := strings.Repeat("x\n", 500)
	d := deliv(storage.ClassHeadlineElements, "2 parts",
		el("run:a::0", "Analysis", body, 500), el("run:b::0", "Summary", "tail", 1))

	var b strings.Builder
	if code := renderElement(&b, d, "run:a::0"); code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(b.String(), body) {
		t.Error("element content was truncated — full retrievability is the contract")
	}

	var b2 strings.Builder
	if code := renderElement(&b2, d, "run:nope::9"); code != ExitBadArgs {
		t.Errorf("unknown element id exit = %d, want %d", code, ExitBadArgs)
	}
}
