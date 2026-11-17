package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/skrptiq/engine/storage"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// GH#1197 (response delivery E3) — `skrptiq show run` renders the engine's headline and element
// list instead of a wall of text.
//
// WHY THE INLINING DECISION IS HERE AND NOT IN THE ENGINE.
//
// Ben's complaint was SCROLLING. Scrolling is a function of the viewport, and the viewport is the
// one fact the engine cannot have — not as a matter of layering taste, but in principle. So E1
// states what is TRUE of the run (this output has two distinct parts; this one is 26 lines) and
// `show` decides what to DISPLAY given a terminal of a particular height.
//
// The alternative was asking the engine to call a 178-byte two-step run "inline" so that it would
// print nicely. That would make `headline_elements` mean "big" rather than "multi-part", and app's
// MCP tools would inherit the lie.
//
// NO THRESHOLD CROSSES THE BOUNDARY. This file reads `Classification.Reason` to tell a parts-only
// split from a length split — "2 parts" versus "26 lines" — and never the numbers behind them. That
// is what keeps acceptance 7 (app, CLI and MCP agree on the same headline and ids) a property of
// one computation rather than a comparison of three.

// partsOnlySuffix is how a parts-only split announces itself. `classifyRun` returns
// `fmt.Sprintf("%d parts", n)` for two-or-more elements and a length- or type-shaped reason for
// everything else, so the suffix is an exact discriminator rather than a guess.
//
// Note that a 7-part 48KB run is ALSO parts-only by reason — which is why the rule below is
// "parts-only AND fits" rather than "parts-only".
const partsOnlySuffix = " parts"

// viewport is what cli knows and the engine does not.
type viewport struct {
	isTerminal bool
	height     int
}

// stdoutViewport reads the real terminal. A pipe or a redirect is NOT a terminal and carries
// everything — a pipe is a console-style caller by definition.
//
// `term.IsTerminal` is the same gate GH#1221 uses for the destination prompt, deliberately: two
// notions of "interactive" in one binary is a defect waiting for a bug report, and the height comes
// from the same package.
func stdoutViewport() viewport {
	fd := os.Stdout.Fd()
	if !term.IsTerminal(fd) {
		return viewport{}
	}
	_, h, err := term.GetSize(fd)
	if err != nil || h <= 0 {
		// A terminal that will not report its size: assume the classic 24 rows rather than
		// assuming infinity. Guessing large here would print a wall, which is the defect.
		return viewport{isTerminal: true, height: 24}
	}
	return viewport{isTerminal: true, height: h}
}

// reservedLines is what the headline, blank lines and footer hint cost before any body prints.
const reservedLines = 6

// bodiesFit reports whether every element body can be shown without scrolling.
//
// Counted on LINES rather than bytes, because the complaint was scrolling. Each body also costs one
// label line and one blank separator.
func bodiesFit(elements []storage.RunElement, height int) bool {
	budget := height - reservedLines
	if budget <= 0 {
		return false
	}
	total := 0
	for _, e := range elements {
		total += e.Lines + 2
		if total > budget {
			return false
		}
	}
	return true
}

// elementLabel disambiguates repeated titles.
//
// Parallel loop members share a node and therefore a title: a four-iteration run prints
// "Competitor Profiling" four times, and a reader cannot tell which is which. Where a title repeats
// it is numbered; where it does not, it is left alone rather than given a pointless "(1/1)".
func elementLabels(elements []storage.RunElement) []string {
	counts := map[string]int{}
	for _, e := range elements {
		counts[e.Title]++
	}
	seen := map[string]int{}
	out := make([]string, len(elements))
	for i, e := range elements {
		if counts[e.Title] > 1 {
			seen[e.Title]++
			out[i] = fmt.Sprintf("%s (%d/%d)", e.Title, seen[e.Title], counts[e.Title])
			continue
		}
		out[i] = e.Title
	}
	return out
}

// renderDelivery is the whole of the display rule. Returns an exit code.
func renderDelivery(w io.Writer, runID string, d *storage.RunDelivery, vp viewport) int {
	// The loud degrade. On the payload rather than in a log precisely so this can be shown, and
	// shown BEFORE the content, because a reader who scrolls past it has not been told.
	if d.Classification.Fallback {
		fmt.Fprintf(w, "note: delivery classification fell back to full output (%s)\n\n",
			d.Classification.FallbackReason)
	}

	// Nothing was produced. This is NOT the plain-text case — it is the absence of one, and
	// printing nothing at all would read as success with an empty answer.
	if len(d.Elements) == 0 {
		fmt.Fprintf(w, "%s\n\n(this run produced no output)\n", d.Headline)
		return ExitOK
	}

	// PLAIN SHORT TEXT, which Ben's 14 November ruling says is fine as it is. No headline, no list
	// header, no element ids, no sizes: a short answer is just the answer.
	if d.Classification.Class == storage.ClassInline {
		for i, e := range d.Elements {
			if i > 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintln(w, strings.TrimRight(e.Content, "\n"))
		}
		return ExitOK
	}

	partsOnly := strings.HasSuffix(d.Classification.Reason, partsOnlySuffix)
	labels := elementLabels(d.Elements)

	// Not a terminal: a pipe or a redirect gets everything, unchanged. Also the honest default when
	// there is no viewport to measure.
	if !vp.isTerminal {
		fmt.Fprintf(w, "%s\n", d.Headline)
		for i, e := range d.Elements {
			fmt.Fprintf(w, "\n%s\n", labels[i])
			fmt.Fprintln(w, strings.TrimRight(e.Content, "\n"))
		}
		return ExitOK
	}

	// THE ACCEPTANCE ROW: a parts-only split that fits the panel prints its CONTENT, not its ids.
	// A 178-byte two-step run on an 80x25 terminal is multi-part and short at the same time, and
	// Ben's sentence does not say which wins — the viewport does.
	if partsOnly && bodiesFit(d.Elements, vp.height) {
		fmt.Fprintf(w, "%s\n", d.Headline)
		for i, e := range d.Elements {
			fmt.Fprintf(w, "\n%s\n", labels[i])
			fmt.Fprintln(w, strings.TrimRight(e.Content, "\n"))
		}
		return ExitOK
	}

	// References: a length split, or more than fits. The sizes are here so a reader can choose, and
	// the full content is always one command away — never truncated, never lost.
	fmt.Fprintf(w, "%s\n\n%s\n", d.Headline, deliveryReason(d.Classification.Reason))
	for i, e := range d.Elements {
		fmt.Fprintf(w, "  %s\n    %s  (%s, %d lines)\n", labels[i], e.ID, byteSize(e.Bytes), e.Lines)
	}
	fmt.Fprintf(w, "\nUse: skrptiq show run %s <element-id> to print one in full.\n", shortRunID(runID))
	return ExitOK
}

// deliveryReason puts the engine's own words in front of the reader rather than inventing a
// paraphrase that could drift from the rule.
func deliveryReason(reason string) string {
	if reason == "" {
		return "Parts:"
	}
	return "Parts (" + reason + "):"
}

func byteSize(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f kB", float64(n)/1024)
}

func shortRunID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// renderElement prints one element in full. Never truncated: "full content is always retrievable"
// is the contract, and this is the command that retrieves it.
func renderElement(w io.Writer, d *storage.RunDelivery, elementID string) int {
	labels := elementLabels(d.Elements)
	for i, e := range d.Elements {
		if e.ID != elementID {
			continue
		}
		fmt.Fprintf(w, "%s\n", labels[i])
		fmt.Fprintf(w, "  %s  (%s, %d lines, %s)\n\n", e.ID, byteSize(e.Bytes), e.Lines, e.Status)
		fmt.Fprintln(w, strings.TrimRight(e.Content, "\n"))
		return ExitOK
	}
	fmt.Fprintf(os.Stderr, "Element not found in this run: %s\n", elementID)
	return ExitBadArgs
}

// runWithDelivery keeps `--json` additive: the pre-GH#1197 run detail is embedded unchanged and the
// delivery sits beside it. A caller that parsed the old shape is unaffected.
type runWithDelivery struct {
	*eng.RunDetail
	Delivery *storage.RunDelivery `json:"delivery"`
}
