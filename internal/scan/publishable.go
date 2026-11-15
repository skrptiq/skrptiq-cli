package scan

import (
	"fmt"
	"sort"

	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// CodeShellStepNotPublishable fires on an execution entry dispatched as a raw
// shell command in a package that declares publish intent. GH#1194 — K-046
// rules raw shell non-publishable and nothing enforced it.
const CodeShellStepNotPublishable = "workflow.shell_step_not_publishable"

const shellContractRef = "K-046 — raw shell is not publishable"

// GH#1194 — K-046's ruling had no gate. The scan is where it goes, and the two
// decisions that matter are both about NOT over-reaching.
//
// WHY THE PREDICATE IS `step_type`, NEVER THE TEXT. Two published packages
// contain the string `local.shell` — `skrpt-builder/prompts/validate-structure.md`
// and `skrptiq-on-skrptiq/prompts/validate-build.md` — in PROSE listing the valid
// step-type vocabulary ("or engine-dispatched: `local.builtin`, `local.shell`,
// `local.http`, …"). Neither executes anything, and `skrptiq-on-skrptiq` is the
// crossing demo's own home. A text-level or file-grep rule would refuse the two
// packages whose purpose is to DOCUMENT the step types. That is GH#858's lesson
// priced in advance: its too-broad read produced 34 false refusals across 10
// working packages, found by the corpus and not by invented cases — which is why
// both packages are corpus fixtures in the tests for this rule.
//
// So the subject is a PARSED execution entry whose `step_type` is `local.shell`,
// keyed exactly where `execution/shell.go: executeShellStep` dispatches. The rule
// and the runtime therefore agree by construction. `context.run` is NOT the
// trigger: `run` without `local.shell` dispatches nothing, and the engine's
// validator already requires `run` FOR a shell step rather than the reverse.
//
// WHY LOCAL-ONLY IS SILENT, NOT `info`. The MVP is local-only and author scripts
// are permitted there (R7), so this must not make the local case unusable. The
// discriminator already exists and is already shipped: `manifest.id` present (or
// a trust block) means publish-bound, and `manifest_id_missing` is already an
// INFO that says "local-only package" in as many words. No new declaration, no
// new flag, and the gate turns on at the exact moment the author declares publish
// intent -- so the crossing demo's `claim-check` keeps working untouched.
//
// An `info` on every local package carrying a shell step would be the soft
// warning users learn to ignore that this repo's conventions forbid, and it would
// be emitted most often to the author for whom it is least relevant. The cost of
// silence is a cliff: works locally for weeks, add `manifest.id`, suddenly an
// error on unchanged content. That is answered IN THE ERROR -- it says the
// content was permitted local-only and that declaring publish intent is what
// changed. One sentence at the transition beats a thousand info lines before it.
//
// Measured over the published catalogue before shipping: ZERO of 100 packages
// declare a `local.shell` execution entry, including all 135 `_shared` objects.
// So this lands with no content wave -- unlike GH#1207, whose blast radius of
// nine forced a repair first. (`local.http` is also zero, measured at the same
// time; it is the same engine-dispatched family and reaches the network, but it
// is a separate question and deliberately carries no rule here.)
func checkShellNotPublishable(pkg *parse.Package, absPath string) []ScanIssue {
	// The discriminator, not a second authority: §1.2 says manifest.id is
	// "absent for local-only packages", and a trust block only exists on a
	// signed bundle. Either means this package is bound for publication.
	publishBound := pkg.Manifest.ID != "" || pkg.Manifest.Trust != nil
	if !publishBound {
		return nil
	}

	var issues []ScanIssue
	for _, wf := range pkg.Nodes {
		if wf.Type != "workflow" {
			continue
		}
		for _, e := range shellEntriesIn(wf.Metadata["execution"]) {
			issues = append(issues, ScanIssue{
				File:     relPath(absPath, wf.FilePath),
				NodeSlug: wf.ID,
				ValidationIssue: storage.ValidationIssue{
					Code:     CodeShellStepNotPublishable,
					Severity: storage.SeverityError,
					Message: fmt.Sprintf(
						"execution entry %q declares step_type local.shell, which is not publishable: the engine dispatches it as `sh -c` with no allowlist, no denylist and no permission model, so anyone installing this package would run its author's commands. "+
							"This content is permitted while the package is local-only — declaring publish intent (manifest.id, or a trust block) is what makes it a refusal, not any change to the step. "+
							"Keep the package local-only, or replace the step with a local.builtin composition.",
						e.label),
					ContractRef:    shellContractRef,
					Field:          "execution",
					ReferencedSlug: e.label,
					ReferenceKind:  "step",
				},
			})
		}
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].NodeSlug != issues[j].NodeSlug {
			return issues[i].NodeSlug < issues[j].NodeSlug
		}
		return issues[i].ReferencedSlug < issues[j].ReferencedSlug
	})
	return issues
}

// shellEntry is one execution entry dispatched as a raw shell command, with the
// best label the entry offers for naming it back to the author.
type shellEntry struct {
	label string
}

// shellEntriesIn walks the execution block for entries whose step_type is
// local.shell, recursing through `parallel:` and any other nesting. The walk
// must recurse: GH#1057 found a duplicate spanning a parallel boundary behaves
// exactly like a flat one, and a top-level-only read would miss a shell step
// hidden one level down -- which is the shape an author would reach for if the
// flat one were refused.
//
// Keys are visited in sorted order so the report is identical run to run; map
// iteration would otherwise randomise which entry is named first.
func shellEntriesIn(v any) []shellEntry {
	switch t := v.(type) {
	case map[string]any:
		var out []shellEntry
		if st, ok := t["step_type"].(string); ok && st == "local.shell" {
			out = append(out, shellEntry{label: shellEntryLabel(t)})
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			if k == "step_type" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, shellEntriesIn(t[k])...)
		}
		return out
	case []any:
		var out []shellEntry
		for _, vv := range t {
			out = append(out, shellEntriesIn(vv)...)
		}
		return out
	}
	return nil
}

// shellEntryLabel names the offending entry the way its author keyed it, so the
// refusal points at something findable in the file. `id:` and `skill:` are the
// two keying forms an execution entry can carry; a label of last resort beats a
// message that names nothing.
func shellEntryLabel(e map[string]any) string {
	if s, ok := e["id"].(string); ok && s != "" {
		return s
	}
	if s, ok := e["skill"].(string); ok && s != "" {
		return s
	}
	return "(unnamed local.shell entry)"
}
