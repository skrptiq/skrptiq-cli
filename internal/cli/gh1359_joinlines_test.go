package cli

import "testing"

// GH#1359 — `joinLines` was `result += line` in a loop, reallocating and copying
// the whole accumulated string every line. The replacement is a single
// `strings.Join`, which is linear.
//
// THIS IS A SEMANTICS TEST, NOT A BENCHMARK. No timing is asserted anywhere: a
// performance assertion in CI is a flake generator, and the property that
// mattered is structural — a single join cannot be quadratic. What needs grading
// is that swapping the implementation changed nothing an existing caller relies
// on, and `joinLines` had no test at all before this, which is why the swap
// needed one.
//
// The trailing newline is the subtle part: the old code returned `result + "\n"`
// unconditionally, so an empty slice produced "\n" rather than "". Both callers
// write the result straight to a manifest, so preserving that exactly is the
// difference between a file that round-trips and one that gains or loses its last
// byte.
func TestGH1359_JoinLines_PreservesTheOldSemantics(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"nil slice still yields a lone newline", nil, "\n"},
		{"empty slice still yields a lone newline", []string{}, "\n"},
		{"one line gains the trailing newline", []string{"id: x"}, "id: x\n"},
		{"two lines are separated and terminated", []string{"a", "b"}, "a\nb\n"},
		{"an empty line is preserved, not collapsed", []string{"a", "", "b"}, "a\n\nb\n"},
		{"a trailing empty line survives", []string{"a", ""}, "a\n\n"},
		{"indentation is untouched", []string{"a:", "  b: 1"}, "a:\n  b: 1\n"},
		{"a carried \\r is left alone", []string{"a\r", "b\r"}, "a\r\nb\r\n"},
	}

	for _, c := range cases {
		if got := joinLines(c.lines); got != c.want {
			t.Errorf("%s: joinLines(%q) = %q, want %q", c.name, c.lines, got, c.want)
		}
	}
}

// The round trip both callers actually perform: read a file, split it, rewrite a
// field, write it back. Anything joinLines does to the bytes it was not asked to
// do lands in a manifest.
func TestGH1359_SplitThenJoin_RoundTripsADocument(t *testing.T) {
	docs := []string{
		"id: a\nname: b\n",
		"id: a\nname: b",     // no trailing newline — gains one, as it always did
		"id: a\n\nname: b\n", // blank line in the middle
		"\n",
		"",
	}
	want := []string{
		"id: a\nname: b\n",
		"id: a\nname: b\n",
		"id: a\n\nname: b\n",
		"\n",
		"\n",
	}
	for i, doc := range docs {
		if got := joinLines(splitLines(doc)); got != want[i] {
			t.Errorf("round trip of %q = %q, want %q", doc, got, want[i])
		}
	}
}
