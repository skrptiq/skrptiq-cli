// gen-smoke-fixtures — offline generator for GH#538 catalogue smoke
// corpus. Reads each skrpt under <hub>/examples/, copies the
// manifest + node-file frontmatter verbatim, strips prompt/skill
// bodies down to lines containing `{{...}}` variable patterns (the
// only body content the engine smoke runner cares about for
// planner + variable-resolution validation), and writes the
// resulting minimal fixture to <out>/<slug>/.
//
// Generation is OFFLINE per the GH#538 plan — App agent re-runs
// this tool whenever the catalogue meaningfully changes, then
// commits the regenerated fixtures to the app repo. CI doesn't
// run this tool.
//
// Usage:
//
//	go run ./cmd/gen-smoke-fixtures \
//	  --hub  ../skrptiq-hub \
//	  --out  ../skrptiq-app/engine/test/fixtures/catalogue \
//	  --limit 0
//
// Flags:
//
//	--hub      Path to the Hub repo containing examples/ (default: ../skrptiq-hub)
//	--out      Output dir (default: ../skrptiq-app/engine/test/fixtures/catalogue)
//	--limit    Limit to first N skrpts (0 = all)
//	--include  Glob: only generate for matching slugs
//	--exclude  Glob: skip matching slugs
//	--v        Verbose per-skrpt logging
//
// Exit: 0 on success (even if some skrpts skipped/failed); 1 only on
// unrecoverable setup errors (hub path missing, out dir uncreateable).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
	"github.com/skrptiq/skrptiq-cli/internal/scan/depresolver"
)

func main() {
	hubPath := flag.String("hub", "../skrptiq-hub", "Path to Hub repo containing examples/")
	outPath := flag.String("out", "../skrptiq-app/engine/test/fixtures/catalogue", "Output directory")
	limit := flag.Int("limit", 0, "Limit to first N skrpts (0 = all)")
	includeGlob := flag.String("include", "", "Glob: only generate for matching slugs")
	excludeGlob := flag.String("exclude", "", "Glob: skip matching slugs")
	verbose := flag.Bool("v", false, "Verbose per-skrpt logging")
	withShared := flag.Bool("with-shared", false,
		"Resolve each skrpt's `dependencies:` block via the CLI scanner's depresolver and synthesise node files for each shared dep node into the consumer fixture. Opt-in (GH#678).")
	hubURL := flag.String("hub-url", "",
		"Hub origin for `/api/shared/<slug>/<v>/metadata` lookups (overrides $SKRPTIQ_HUB_URL; falls back to depresolver default)")
	cacheDir := flag.String("cache-dir", "",
		"Dep-resolver cache dir override (falls back to ~/.skrptiq/cache)")
	flag.Parse()

	examplesDir := filepath.Join(*hubPath, "examples")
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read hub examples: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*outPath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create out dir: %v\n", err)
		os.Exit(1)
	}

	// Build the dep resolver up-front when --with-shared is set so all
	// generations share one cache instance (and one underlying HTTP
	// client). Resolution failures during a per-skrpt run fail that
	// skrpt loud, per the plan's no-silent-degradation invariant.
	var resolver *depresolver.Resolver
	if *withShared {
		resolved := *hubURL
		if resolved == "" {
			resolved = os.Getenv("SKRPTIQ_HUB_URL")
		}
		r, err := depresolver.New(depresolver.Config{
			HubBaseURL: resolved,
			CacheDir:   *cacheDir,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "init dep resolver: %v\n", err)
			os.Exit(1)
		}
		resolver = r
	}

	var generated, skipped, failed int
	var failures []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		// _shared/ hosts standalone shared bundles; the smoke runner
		// targets consumer skrpts that have an execution block, not
		// dep targets. Skip the whole directory.
		if slug == "_shared" {
			continue
		}
		if !slugMatches(slug, *includeGlob, *excludeGlob) {
			skipped++
			if *verbose {
				fmt.Fprintf(os.Stderr, "skip %s (glob filter)\n", slug)
			}
			continue
		}
		if *limit > 0 && generated >= *limit {
			break
		}

		srcDir := filepath.Join(examplesDir, slug)
		dstDir := filepath.Join(*outPath, slug)
		if err := generateFixture(srcDir, dstDir, resolver); err != nil {
			failed++
			failures = append(failures, fmt.Sprintf("%s: %v", slug, err))
			// Roll back the partial output so reruns are deterministic.
			_ = os.RemoveAll(dstDir)
			if *verbose {
				fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", slug, err)
			}
			continue
		}
		generated++
		if *verbose {
			fmt.Fprintf(os.Stderr, "ok   %s\n", slug)
		}
	}

	fmt.Fprintf(os.Stderr, "generated=%d skipped=%d failed=%d\n", generated, skipped, failed)
	if failed > 0 {
		fmt.Fprintln(os.Stderr, "failures:")
		for _, f := range failures {
			fmt.Fprintf(os.Stderr, "  %s\n", f)
		}
	}
}

// slugMatches returns true iff slug passes both include and exclude
// filters. Empty globs are no-ops.
func slugMatches(slug, includeGlob, excludeGlob string) bool {
	if includeGlob != "" && !globMatch(slug, includeGlob) {
		return false
	}
	if excludeGlob != "" && globMatch(slug, excludeGlob) {
		return false
	}
	return true
}

func globMatch(s, pattern string) bool {
	re := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, `.*`) + "$"
	matched, _ := regexp.MatchString(re, s)
	return matched
}

// generateFixture produces a minimal fixture at dstDir from the
// source skrpt at srcDir. Returns nil on success; an error on any
// IO, parse, or validation failure (caller cleans up dstDir on error).
//
// resolver is non-nil iff --with-shared was set. When non-nil, the
// consumer's `dependencies:` block is resolved through the CLI
// scanner's depresolver and each dep node summary is materialised as
// a synthetic node file inside the consumer fixture. Local files win
// on slug collision (mirrors engine ValidateWorkflowWithDeps).
func generateFixture(srcDir, dstDir string, resolver *depresolver.Resolver) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dstDir, err)
	}

	// Manifest copied verbatim. The smoke runner needs the full
	// dependencies + execution-ref shape to drive the planner; we
	// don't strip discovery fields like description/tags because they
	// don't add measurable bulk.
	manifestSrc := filepath.Join(srcDir, "skrptiq.yaml")
	manifestDst := filepath.Join(dstDir, "skrptiq.yaml")
	if err := copyFile(manifestSrc, manifestDst); err != nil {
		return fmt.Errorf("copy manifest: %w", err)
	}

	// Walk node-type subdirectories. The engine's parse.ReadPackage
	// recognises a fixed set; we mirror that.
	nodeDirs := []string{
		"workflows", "skills", "prompts",
		"documents", "sources", "services", "assets",
	}
	for _, dir := range nodeDirs {
		srcSub := filepath.Join(srcDir, dir)
		entries, err := os.ReadDir(srcSub)
		if err != nil {
			// Missing dir is fine — not every skrpt has every node type.
			continue
		}
		dstSub := filepath.Join(dstDir, dir)
		if err := os.MkdirAll(dstSub, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dstSub, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			srcFile := filepath.Join(srcSub, e.Name())
			dstFile := filepath.Join(dstSub, e.Name())
			if err := generateNodeFile(srcFile, dstFile); err != nil {
				return fmt.Errorf("node %s/%s: %w", dir, e.Name(), err)
			}
		}
	}

	// Resolve + synthesise shared dep nodes when --with-shared is on.
	// Done BEFORE validation so the validator sees the full fixture
	// shape (including shared nodes the workflow `uses`).
	if resolver != nil {
		if err := materialiseSharedDeps(dstDir, resolver); err != nil {
			return fmt.Errorf("shared deps: %w", err)
		}
	}

	// Validate the generated fixture parses cleanly via the same
	// canonical reader the scanner uses. This catches generation
	// bugs (mangled frontmatter, missing required fields) at gen
	// time rather than letting the smoke runner crash on them.
	if _, issues, err := parse.ReadPackage(dstDir); err != nil {
		return fmt.Errorf("validate: parse.ReadPackage: %w", err)
	} else if errs := filterErrors(issues); len(errs) > 0 {
		first := errs[0]
		return fmt.Errorf("validate: %d parse error(s); first: %s (%s:%d) %s",
			len(errs), first.Code, first.File, first.Line, first.Message)
	}
	return nil
}

// nodeTypeToDir maps an engine node type to its standard directory
// inside a skrpt package. Mirrors the reverse of parse.DirToType
// (engine doesn't expose the forward direction yet).
var nodeTypeToDir = map[string]string{
	"workflow": "workflows",
	"skill":    "skills",
	"prompt":   "prompts",
	"service":  "services",
	"source":   "sources",
	"document": "documents",
	"asset":    "assets",
}

// materialiseSharedDeps reads dstDir's already-written manifest, walks
// the declared dependencies through the depresolver, and writes a
// synthesised node file for each returned DepNodeSummary inside the
// consumer fixture. Local files win on slug collision (the depresolver
// caller doesn't need to know this — we just skip when the file
// already exists).
//
// On any resolver failure (missing Hub, missing cache, checksum
// mismatch, fetch error), this returns the error so the caller fails
// the whole fixture loud. Silent half-resolution is the failure mode
// GH#678's "no silent degradation" rule guards against.
func materialiseSharedDeps(dstDir string, resolver *depresolver.Resolver) error {
	pkg, _, err := parse.ReadPackage(dstDir)
	if err != nil {
		return fmt.Errorf("re-parse: %w", err)
	}
	if pkg.Dependencies == nil || len(pkg.Dependencies) == 0 {
		return nil
	}
	res := resolver.Resolve(pkg.Dependencies)
	for _, issue := range res.Issues {
		if issue.Severity == "error" {
			return fmt.Errorf("dep resolution: %s: %s", issue.Code, issue.Message)
		}
	}
	for _, dn := range res.DepNodes {
		if err := writeSyntheticNode(dstDir, dn); err != nil {
			return fmt.Errorf("synth %s/%s: %w", dn.Type, dn.Slug, err)
		}
	}
	return nil
}

// writeSyntheticNode materialises one DepNodeSummary as a node file
// under dstDir. Skips when a local file at the same slug already
// exists (mirrors engine's depNodes-merge "local wins" semantics).
func writeSyntheticNode(dstDir string, dn storage.DepNodeSummary) error {
	dir, ok := nodeTypeToDir[dn.Type]
	if !ok {
		// Unrecognised node type — skip rather than fail. The engine
		// validator catches anything material; we shouldn't invent a
		// directory the engine doesn't recognise.
		return nil
	}
	subDir := filepath.Join(dstDir, dir)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(subDir, dn.Slug+".md")
	// Local wins on slug collision (engine depNodes-merge semantics).
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	body := dn.Content
	if strings.TrimSpace(body) == "" {
		body = "(stripped for smoke corpus)\n"
	} else if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	// Frontmatter mirrors what engine NodeFile expects: id, type,
	// title, description. fileSlug is implicit from the filename so
	// we don't emit it in the frontmatter.
	front := fmt.Sprintf(
		"id: %q\ntype: %q\ntitle: %q\ndescription: %q\n",
		dn.ID, dn.Type, dn.Title,
		fmt.Sprintf("(synthesised from hub-shared/%s)", dn.Slug),
	)
	out := "---\n" + front + "---\n" + body
	return os.WriteFile(dst, []byte(out), 0o644)
}

// generateNodeFile reads srcFile, preserves the frontmatter verbatim,
// strips the body down to lines containing `{{variable}}` patterns,
// and writes the result to dstFile.
func generateNodeFile(srcFile, dstFile string) error {
	raw, err := os.ReadFile(srcFile)
	if err != nil {
		return err
	}
	front, body, ok := splitFrontmatter(raw)
	if !ok {
		// No frontmatter delimiters — copy verbatim. parse will
		// surface anything that breaks downstream.
		return os.WriteFile(dstFile, raw, 0o644)
	}
	strippedBody := stripBodyToVariablePatterns(body)
	out := "---\n" + front + "---\n" + strippedBody
	return os.WriteFile(dstFile, []byte(out), 0o644)
}

// splitFrontmatter splits a node file into (frontmatter, body) at the
// `---` delimiters. Returns ok=false if the file doesn't start with `---`
// or doesn't have a closing `---`.
func splitFrontmatter(raw []byte) (front, body string, ok bool) {
	s := string(raw)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return "", "", false
	}
	rest := strings.TrimPrefix(s, "---\n")
	rest = strings.TrimPrefix(rest, "---\r\n")
	// Find the closing `---` on its own line.
	endIdx := strings.Index(rest, "\n---\n")
	if endIdx < 0 {
		endIdx = strings.Index(rest, "\n---\r\n")
	}
	if endIdx < 0 {
		return "", "", false
	}
	return rest[:endIdx+1], rest[endIdx+len("\n---\n"):], true
}

var varPattern = regexp.MustCompile(`\{\{[^}]+\}\}`)

// stripBodyToVariablePatterns keeps only the lines containing at
// least one `{{...}}` reference. The smoke runner cares about variable
// resolution at plan-build time, not prose content. If the body has
// no variables, we keep a single placeholder line so the parser sees
// non-empty content (some parsers reject zero-body prompt/skill files).
func stripBodyToVariablePatterns(body string) string {
	var keep []string
	for _, line := range strings.Split(body, "\n") {
		if varPattern.MatchString(line) {
			keep = append(keep, strings.TrimRight(line, "\r"))
		}
	}
	if len(keep) == 0 {
		return "(stripped for smoke corpus)\n"
	}
	return strings.Join(keep, "\n") + "\n"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// filterErrors returns just the SeverityError issues from a parse pass.
// Warnings and infos are noise for the gen tool's validate step.
func filterErrors(issues []manifest.ParseIssue) []manifest.ParseIssue {
	var out []manifest.ParseIssue
	for _, i := range issues {
		if i.Severity == manifest.SeverityError {
			out = append(out, i)
		}
	}
	return out
}
