package scan

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// bridgeResult holds the output of bridgeToHydration — everything the
// scanner needs to feed HydratePackage and attribute issues to files.
type bridgeResult struct {
	Input     storage.HydrationInput
	Issues    []ScanIssue
	NodeFiles map[string]string      // nodeID → relative file path
	EdgeFiles map[string]edgeFileCtx // edgeID → file context
}

// depContext carries dep-resolution state into the bridge so that
// connection edges with dep-provided targets resolve without
// surfacing as errors (GH#630 plan v2 + GH#650).
//
// Zero value means "legacy bundle mode" — no dependencies block, no
// dep resolution, three-tier collapses to its first tier (local-only
// with legacy code on miss).
type depContext struct {
	// Summaries maps dep-provided slug → its DepNodeSummary. The bridge
	// hydrates a synthetic NodeInput for each summary (so engine's edge
	// FK + uses-target checks resolve through it) and rewrites any
	// connection edge whose target hits a summary slug to use the
	// summary's UUID as TargetID (so engine's `usesTargets[node.ID]`
	// matches the dep-merged slugToNode entry).
	Summaries map[string]storage.DepNodeSummary
	// HasDepsBlock is true iff pkg.Dependencies != nil — i.e. the
	// manifest declared a `dependencies:` block (even if empty). The
	// discriminator for "re-code unresolved as dependency.unresolved_slug"
	// (true) vs "keep legacy scan.edge_target_unresolved code" (false).
	HasDepsBlock bool
	// Permissive is true under `--no-resolve-deps` when HasDepsBlock is
	// also true — i.e. the author has opted into structural-only
	// validation. Per plan v1 §4 ("accepted as potentially dep-provided
	// — no error, no fetch"), the scanner treats every non-local
	// reference as if it were dep-provided rather than emitting either
	// the legacy code or the re-coded dependency.unresolved_slug.
	Permissive bool
}

// bridgeToHydration converts a parse.Package into a storage.HydrationInput
// plus any scan-level issues (cross-package edges, unresolved targets).
//
// GH#530 Phase 3b: the local-nodes-and-edges portion is delegated to
// storage.HydrationInputFrom — the canonical engine-side translation
// every consumer (App workspace, App Hub import, CLI scanner) now
// shares. The bridge keeps responsibility for:
//
//   - The CLI-specific namespace fallback for unnamed packages.
//   - Setting HubImportID = "scan-import".
//   - Building NodeFiles / EdgeFiles maps so the scanner can attribute
//     issues back to source files.
//   - Dep-aware enrichment that depends on the depresolver's runtime
//     state: synth NodeInputs for dep summaries, TargetID rewriting on
//     connection edges that hit a dep-provided slug, and the four-tier
//     error classification for unresolved / cross-package / permissive
//     edges.
//
// deps carries dep-resolution context (GH#630 plan v2). Pass
// depContext{} for legacy / no-deps bundles.
func bridgeToHydration(pkg parse.Package, absPath string, deps depContext) bridgeResult {
	// 1. Engine builds the trivial local-only base.
	input, _ := storage.HydrationInputFrom(pkg)

	// 2. CLI-specific manifest fixups.
	if input.Manifest == nil {
		input.Manifest = make(map[string]any)
	}
	if input.Manifest["name"] == nil || input.Manifest["name"] == "" {
		input.Manifest["name"] = filepath.Base(absPath)
	}
	input.HubImportID = "scan-import"

	// 3. Local node set (for the connection walk below) and file-path
	// attribution map (the engine helper doesn't surface paths).
	nodeIDSet := make(map[string]bool, len(pkg.Nodes))
	nodeFiles := make(map[string]string, len(pkg.Nodes))
	for _, nf := range pkg.Nodes {
		nodeIDSet[nf.ID] = true
		nodeFiles[nf.ID] = relPath(absPath, nf.FilePath)
	}

	// 4. Dep-aware enrichment: synthetic NodeInputs for each dep
	// summary whose slug isn't shadowed by a local node (local wins on
	// slug collision, mirroring engine's depNodes-merge semantics).
	// Synth nodes are needed in the temp DB so (a) connection-edge FK
	// validation against a dep target satisfies and (b) engine's
	// `usesTargets[node.ID]` resolves against the dep target's UUID.
	for slug, summary := range deps.Summaries {
		if nodeIDSet[slug] {
			continue // local slug wins
		}
		synthSlug := summary.Slug
		var synthContent *string
		if summary.Content != "" {
			c := summary.Content
			synthContent = &c
		}
		input.Nodes = append(input.Nodes, storage.NodeInput{
			ID:       summary.ID,
			Type:     summary.Type,
			Title:    summary.Title,
			Content:  synthContent,
			FileSlug: &synthSlug,
		})
	}

	// 5. Walk connections to (a) classify cross-package + unresolved
	// targets the engine helper skipped, (b) emit dep-aware edges with
	// rewritten TargetIDs, and (c) build the EdgeFiles map for ALL
	// emitted edges (both the engine-built local ones and the
	// CLI-emitted dep-rewritten ones).
	var issues []ScanIssue
	edgeFiles := make(map[string]edgeFileCtx)

	for _, nf := range pkg.Nodes {
		rel := relPath(absPath, nf.FilePath)
		for _, conn := range nf.Connections {
			edgeID := fmt.Sprintf("%s--%s--%s", nf.ID, conn.Type, conn.Target)

			// Cross-package edge: target contains "/" (namespace/slug).
			// Legal at runtime but unresolvable in a single-package scan.
			if strings.Contains(conn.Target, "/") {
				issues = append(issues, ScanIssue{
					File:     rel,
					NodeSlug: nf.ID,
					ValidationIssue: storage.ValidationIssue{
						Code:     "scan.cross_package_edge",
						Severity: storage.SeverityWarning,
						Message:  fmt.Sprintf("cross-package edge target %q — will be resolved at runtime", conn.Target),
						Field:    "connections",
					},
				})
				continue
			}

			// Local target: engine helper already emitted the edge.
			// Just record file attribution.
			if nodeIDSet[conn.Target] {
				edgeFiles[edgeID] = edgeFileCtx{file: rel, sourceSlug: nf.ID}
				continue
			}

			// Non-local target — four-tier handling matching GH#630
			// plan v2 + GH#650:
			//   1. dep-provided slug → emit edge with TargetID
			//      rewritten to the dep summary's UUID (the synth
			//      node hydrated above satisfies FK + usesTargets).
			//   2. --no-resolve-deps + has-deps-block → drop, treat
			//      as potentially dep-provided.
			//   3. non-local + has-deps-block → re-code as
			//      dependency.unresolved_slug.
			//   4. non-local + no deps block → keep legacy
			//      scan.edge_target_unresolved (no regression).
			if summary, hit := deps.Summaries[conn.Target]; hit {
				input.Edges = append(input.Edges, storage.EdgeInput{
					ID:       edgeID,
					SourceID: nf.ID,
					TargetID: summary.ID,
					Type:     conn.Type,
					Position: conn.Position,
				})
				edgeFiles[edgeID] = edgeFileCtx{file: rel, sourceSlug: nf.ID}
				continue
			}
			if deps.Permissive {
				continue
			}
			code := "scan.edge_target_unresolved"
			message := fmt.Sprintf("connection target %q not found", conn.Target)
			if deps.HasDepsBlock {
				code = "dependency.unresolved_slug"
				message = fmt.Sprintf("connection target %q is not local and is not provided by any declared dependency", conn.Target)
			}
			issues = append(issues, ScanIssue{
				File:     rel,
				NodeSlug: nf.ID,
				ValidationIssue: makeIssue(code, "error", message, "§6.4", "connections"),
			})
		}
	}

	return bridgeResult{
		Input:     input,
		Issues:    issues,
		NodeFiles: nodeFiles,
		EdgeFiles: edgeFiles,
	}
}

// parseIssuesToScanIssues converts engine manifest.ParseIssue records
// into the CLI's ScanIssue type, mapping severity strings along the way.
func parseIssuesToScanIssues(issues []manifest.ParseIssue) []ScanIssue {
	out := make([]ScanIssue, 0, len(issues))
	for _, pi := range issues {
		out = append(out, ScanIssue{
			File: pi.File,
			ValidationIssue: storage.ValidationIssue{
				Code:     string(pi.Code),
				Severity: parseToStorageSeverity(pi.Severity),
				Message:  pi.Message,
			},
		})
	}
	return out
}

// parseToStorageSeverity maps manifest.Severity ("warn") to
// storage.ValidationSeverity ("warning"). The vocabulary difference is
// a known drift surface (W-CLI-1); Wave 4 F8 consolidates it.
func parseToStorageSeverity(s manifest.Severity) storage.ValidationSeverity {
	switch s {
	case manifest.SeverityError:
		return storage.SeverityError
	case manifest.SeverityWarn:
		return storage.SeverityWarning
	case manifest.SeverityInfo:
		return storage.SeverityInfo
	default:
		return storage.SeverityWarning
	}
}

// relPath returns the path of full relative to base.
func relPath(base, full string) string {
	rel, err := filepath.Rel(base, full)
	if err != nil {
		return full
	}
	return rel
}

