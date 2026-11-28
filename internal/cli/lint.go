package cli

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/parse"
)

// Lint handles `skrptiq lint <dir> [--auto-fix]`.
// Surfaces fixable identity issues; --auto-fix rewrites in place.
func Lint(args []string) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	autoFix := fs.Bool("auto-fix", false, "Rewrite fixable issues in place")
	if err := fs.Parse(args); err != nil {
		return ExitBadArgs
	}

	dir := fs.Arg(0)
	if dir == "" {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq lint <dir> [--auto-fix]")
		return ExitBadArgs
	}

	pkg, issues, err := parse.ReadPackage(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	if *autoFix {
		return lintAutoFix(dir, pkg, issues)
	}

	return lintReport(issues)
}

// lintReport prints issues and returns the appropriate exit code.
func lintReport(issues []manifest.ParseIssue) int {
	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return ExitOK
	}

	for _, issue := range issues {
		fmt.Printf("  %s  %s  %s\n", severityLabel(issue.Severity), issue.Code, issue.Message)
		if issue.Suggestion != "" {
			fmt.Printf("         suggestion: %s\n", issue.Suggestion)
		}
	}

	unrecognised, errors, warnings := lintTally(issues)
	for _, line := range unrecognised {
		fmt.Printf("  %s  %s  %s\n", severityLabel(manifest.SeverityError), CodeUnrecognisedSeverity, line)
	}

	return lintExitCode(errors, warnings)
}

// CodeUnrecognisedSeverity is lint's counterpart to the scan's code of the same
// name (GH#1358). Two vocabularies, one defect: the scan's came from
// `storage.ValidationSeverity`, lint's from `manifest.Severity`.
const CodeUnrecognisedSeverity = "lint.unrecognised_severity"

// lintTally counts issues by severity and returns a message for every severity
// lint does not recognise (GH#1367).
//
// THE SAME FAIL-OPEN AS GH#1358, ONE COMMAND OVER. This was a bare `switch` over
// two severity constants with no `default`, while the loop above prints every
// issue unconditionally and `severityLabel` has a `default` that renders an
// unknown severity's raw string. So an issue carrying a third value was PRINTED,
// counted toward nothing, and `lint` returned `ExitOK` — a finding on stdout with
// a success status, indistinguishable from a clean lint to anything that checks
// the status.
//
// The printer had a default and the counter one line away did not, and the counter
// is the one that decided the exit code. Found by sweeping for that shape after
// fixing it in the scan, which is the only reason anyone looked.
//
// THE EXPOSURE IS THE ENGINE'S VOCABULARY, not cli's. These severities arrive on
// `manifest.ParseIssue` from `parse.ReadPackage` and are passed through untouched,
// so lint's exit code was correct only for as long as `manifest.Severity` stayed
// at three values. Note `SeverityInfo` is deliberately uncounted — info is not a
// verdict input — and that is a decision about a KNOWN value, which is exactly
// what distinguishes it from the gap.
//
// It names the value rather than promoting it silently, for GH#1358's reason: a
// silent promotion is a second guess the next reader cannot see.
func lintTally(issues []manifest.ParseIssue) (unrecognised []string, errors, warnings int) {
	counts := map[string]int{}
	example := map[string]string{}

	for _, issue := range issues {
		switch issue.Severity {
		case manifest.SeverityError:
			errors++
		case manifest.SeverityWarn:
			warnings++
		case manifest.SeverityInfo:
			// Counted toward no verdict ON PURPOSE: info is informational, and
			// that is a statement about a known value rather than a gap.
		default:
			raw := string(issue.Severity)
			counts[raw]++
			if _, seen := example[raw]; !seen {
				example[raw] = string(issue.Code)
			}
		}
	}

	for _, sev := range sortedSeverityKeys(counts) {
		unrecognised = append(unrecognised, fmt.Sprintf(
			"%d finding(s) above carry the severity %q, which lint does not recognise "+
				"(it knows %q, %q and %q) — for example %q. They were printed but counted toward "+
				"no verdict, so this lint would have exited 0 with findings on screen. Refusing "+
				"instead: an unrecognised severity means lint and whatever produced the finding "+
				"disagree about what counts as a failure",
			counts[sev], sev,
			string(manifest.SeverityError), string(manifest.SeverityWarn), string(manifest.SeverityInfo),
			example[sev]))
		errors++
	}

	return unrecognised, errors, warnings
}

// sortedSeverityKeys keeps the unrecognised-severity output deterministic.
func sortedSeverityKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// lintExitCode is the single place lint's verdict is derived.
//
// NOTE THAT `ExitFailed` IS 1 AND THE WARNING CODE IS ALSO 1: lint's exit
// vocabulary is two-valued, unlike the scan's 2/1/0. So from outside, an error and
// a warning are indistinguishable, and the only observable fail-open was 0 instead
// of 1. Said here because asserting "error -> ExitFailed" and "warning -> 1"
// separately is asserting the same number twice, and a reader should know that
// rather than infer agreement from a passing test.
func lintExitCode(errors, warnings int) int {
	if errors > 0 {
		return ExitFailed
	}
	if warnings > 0 {
		return 1
	}
	return ExitOK
}

// lintAutoFix applies MigrateIdentity to the manifest and rewrites.
func lintAutoFix(dir string, pkg parse.Package, issues []manifest.ParseIssue) int {
	if pkg.Manifest.ID == "" {
		return lintReport(issues)
	}

	canonical, migIssue, err := manifest.MigrateIdentity(pkg.Manifest.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot auto-fix id: %v\n", err)
		return ExitFailed
	}

	if canonical == pkg.Manifest.ID {
		fmt.Println("manifest.id is already canonical — no changes needed.")
		return lintReport(issues)
	}

	// Rewrite skrptiq.yaml: read, replace the old ID, write back.
	yamlPath := dir + "/skrptiq.yaml"
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	// Simple string replacement — the ID appears as `id: <value>` in YAML.
	updated := replaceYAMLField(string(data), "id", canonical)
	if err := os.WriteFile(yamlPath, []byte(updated), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	if migIssue != nil {
		fmt.Printf("  Fixed: %s\n", migIssue.Message)
		if migIssue.Suggestion != "" {
			fmt.Printf("  Rewritten to: %s\n", migIssue.Suggestion)
		}
	}
	fmt.Printf("  manifest.id updated to %s\n", canonical)
	return ExitOK
}

func severityLabel(s manifest.Severity) string {
	switch s {
	case manifest.SeverityError:
		return "ERROR  "
	case manifest.SeverityWarn:
		return "WARNING"
	case manifest.SeverityInfo:
		return "INFO   "
	default:
		return string(s)
	}
}

// replaceYAMLField rewrites the FIRST line whose space/tab-trimmed text begins
// `<field>:`, and returns the document unchanged if no line does.
//
// IT IS NOT RESTRICTED TO A TOP-LEVEL FIELD, to a mapping key, or to YAML
// structure at all, and this comment used to claim it was (GH#1359). It matches a
// nested mapping key, and a line of prose inside a block scalar, before it reaches
// a top-level key further down the file — so on such a manifest it rewrites the
// wrong line while the caller reports the right one. A list item (`- id:`) is NOT
// matched, because the trimmed line still begins `- `.
//
// GH#1331 dimension 2 has the cases and the end-to-end runs. The BEHAVIOUR is
// unfixed and out of scope here; what is fixed is a description that made the
// defect hard to see. Anchor the surgery to what the parser reported, not to a
// second search with weaker rules.
func replaceYAMLField(content, field, newValue string) string {
	lines := splitLines(content)
	prefix := field + ":"
	for i, line := range lines {
		trimmed := trimLeftSpace(line)
		if len(trimmed) >= len(prefix) && trimmed[:len(prefix)] == prefix {
			// Preserve leading whitespace.
			indent := line[:len(line)-len(trimmed)]
			lines[i] = indent + field + ": " + newValue
			break
		}
	}
	return joinLines(lines)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinLines rejoins split lines and restores the trailing newline.
//
// `strings.Join` rather than `result += line` in a loop (GH#1359). The old shape
// reallocated and copied the whole accumulated string on every line, so it was
// quadratic in the file: 20,000 lines took 2.01s against 837µs for a single
// join, and 40,000 took 9.20s. Every manifest rewrite passes through here, so
// that was ~80% of `migrate-identity`'s wall-clock on a 1.3MB manifest.
//
// NO BENCHMARK GUARDS THIS, deliberately. A timing assertion in CI is a flake
// generator, and the property is structural rather than measured: a single join
// cannot be quadratic. The numbers above are the record.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

func trimLeftSpace(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return s[i:]
		}
	}
	return ""
}
