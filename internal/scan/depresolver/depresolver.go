// Package depresolver resolves the node-slug membership of a skrpt's
// declared dependencies for scanner-time workflow-execution validation.
//
// Background: GH#630. Under the dep-referenced model (GH#612), a consumer
// bundle's `workflow.execution[].skill` / `.prompt` refs may resolve to
// nodes that live in a declared dep rather than the local package. The
// scanner needs to know which slugs each dep provides so the three-tier
// resolution (local → dep-provided → ERROR) can fire.
//
// Identity (K-037, GH#638): every dep ref carries an immutable UUID
// (`ID`) plus a logical slug (`Name`). UUID is the canonical identity
// for cache keying + comparison; slug is for URL routing and human
// readability only. No `hub-shared/` prefix anywhere — that namespace
// convention is gone.
//
// Trust chain (plan §3; GH#722): the resolver fetches metadata from Hub
// and asserts the full identity triple — the Hub-reported checksum, id
// (canonical K-037 UUID), and version must each equal the manifest-
// declared value — before accepting the returned node list. Any mismatch
// is a hard ERROR (`dependency.{checksum,id,version}_mismatch`) — closes
// the CTO §F2.1 phantom-dependency hole and the §6.2c checksum-only-
// identity hole (the metadata URL keys on name+version, not id, so
// checksum match alone does not prove the object is the declared one).
// There is no fallback path; if verification fails the scan fails.
package depresolver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// DefaultHubBaseURL is used when Config.HubBaseURL is empty.
const DefaultHubBaseURL = "https://hub.skrptiq.ai"

// Config controls resolver behaviour.
type Config struct {
	// HubBaseURL overrides the catalogue origin. Empty → DefaultHubBaseURL.
	// Tests inject an httptest.Server.URL here.
	HubBaseURL string
	// CacheDir is the directory holding dep-nodes.json. Empty → ~/.skrptiq/cache.
	CacheDir string
	// HTTPClient overrides the HTTP client. nil → 30s-timeout default.
	HTTPClient *http.Client
}

// Resolver fetches, verifies, and caches the slug list each declared dep
// provides.
type Resolver struct {
	hubBaseURL string
	cacheDir   string
	httpCli    *http.Client
}

// New constructs a Resolver from cfg, applying defaults.
func New(cfg Config) (*Resolver, error) {
	base := cfg.HubBaseURL
	if base == "" {
		base = DefaultHubBaseURL
	}
	base = strings.TrimRight(base, "/")

	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		cacheDir = filepath.Join(home, ".skrptiq", "cache")
	}

	cli := cfg.HTTPClient
	if cli == nil {
		cli = &http.Client{Timeout: 30 * time.Second}
	}

	return &Resolver{
		hubBaseURL: base,
		cacheDir:   cacheDir,
		httpCli:    cli,
	}, nil
}

// Result is what Resolve returns.
type Result struct {
	// ProvidedSlugs maps slug → the dep slug providing it (e.g.
	// "llm-service" → "llm-service"). Last-wins on multi-dep collision
	// (rare given one-bundle-one-slug convention).
	ProvidedSlugs map[string]string
	// DepNodes is the flattened set of dep node summaries from every
	// resolved dependency, in the shape engine.storage.ValidateWorkflowWithDeps
	// expects (GH#650). Pass-through to the validator so binding /
	// for_each / plumbing checks resolve through K-037 dep references.
	DepNodes []storage.DepNodeSummary
	// Issues are per-dep validation issues with file context attached by
	// the caller (the manifest path).
	Issues []storage.ValidationIssue
}

// Resolve fetches metadata for each dep, verifies checksums against the
// manifest, and returns the merged slug set. Cache hits short-circuit
// the fetch. Issues are accumulated per dep; one failed dep does not
// abort resolution of the others (the scanner should surface all
// problems in a single pass).
func (r *Resolver) Resolve(deps []parse.DependencyRef) Result {
	res := Result{ProvidedSlugs: map[string]string{}}
	if len(deps) == 0 {
		return res
	}

	cache, _ := r.readCache() // miss/corruption → empty cache, never fatal

	dirty := false
	for _, dep := range deps {
		entry, issues := r.resolveDep(dep, cache)
		res.Issues = append(res.Issues, issues...)
		if entry == nil {
			continue
		}
		key := cacheKey(dep)
		if _, hit := cache[key]; !hit {
			cache[key] = *entry
			dirty = true
		}
		for _, n := range entry.Nodes {
			res.ProvidedSlugs[n.Slug] = dep.Name
			res.DepNodes = append(res.DepNodes, storage.DepNodeSummary{
				ID:      n.ID,
				Slug:    n.Slug,
				Type:    n.Type,
				Title:   n.Title,
				Content: n.Content,
			})
		}
	}

	if dirty {
		if err := r.writeCache(cache); err != nil {
			res.Issues = append(res.Issues, storage.ValidationIssue{
				Code:     "dependency.cache_write_failed",
				Severity: storage.SeverityWarning,
				Message:  fmt.Sprintf("could not persist dep-nodes cache: %v", err),
			})
		}
	}

	return res
}

// resolveDep returns either a populated cache entry (hit or fresh fetch
// + verified) or nil if resolution failed. Failure issues are returned
// alongside.
func (r *Resolver) resolveDep(dep parse.DependencyRef, cache map[string]cacheEntry) (*cacheEntry, []storage.ValidationIssue) {
	if entry, hit := cache[cacheKey(dep)]; hit {
		return &entry, deprecationIssues(dep, entry)
	}

	meta, err := r.fetchMetadata(dep)
	if err != nil {
		return nil, []storage.ValidationIssue{{
			Code:     "dependency.fetch_failed",
			Severity: storage.SeverityError,
			Message:  fmt.Sprintf("dependency %s@%s: %v", dep.Name, dep.Version, err),
			Field:    "dependencies",
		}}
	}

	if meta.Checksum != dep.Checksum {
		return nil, []storage.ValidationIssue{{
			Code:     "dependency.checksum_mismatch",
			Severity: storage.SeverityError,
			Message:  fmt.Sprintf("dependency %s@%s: manifest checksum %s != Hub-reported %s", dep.Name, dep.Version, dep.Checksum, meta.Checksum),
			Field:    "dependencies",
		}}
	}

	// §6.2c identity assertion (GH#722): checksum match alone is not
	// identity. The metadata URL is keyed on name+version, so a manifest
	// could declare dep.ID = <UUID-A> while name/version resolve to a
	// *different* catalogue object whose checksum happens to match. Assert
	// the Hub-reported canonical UUID equals the declared one — reject,
	// never warn. This is the online-path sibling of the GH#714 offline
	// gate; both trust paths now assert id+version+checksum.
	if meta.ID != dep.ID {
		return nil, []storage.ValidationIssue{{
			Code:     "dependency.id_mismatch",
			Severity: storage.SeverityError,
			Message:  fmt.Sprintf("dependency %s@%s: manifest id %s != Hub-reported %s", dep.Name, dep.Version, dep.ID, meta.ID),
			Field:    "dependencies",
		}}
	}

	// Belt-and-braces: the URL keys on version so this is near-redundant,
	// but a Hub returning a divergent version must fail loud, not pass.
	if meta.Version != dep.Version {
		return nil, []storage.ValidationIssue{{
			Code:     "dependency.version_mismatch",
			Severity: storage.SeverityError,
			Message:  fmt.Sprintf("dependency %s@%s: manifest version %s != Hub-reported %s", dep.Name, dep.Version, dep.Version, meta.Version),
			Field:    "dependencies",
		}}
	}

	entry := cacheEntry{
		ID:           dep.ID,
		Version:      dep.Version,
		Checksum:     dep.Checksum,
		Nodes:        meta.Nodes,
		FetchedAt:    time.Now().UTC().Format(time.RFC3339),
		Deprecated:   meta.Deprecated,
		SupersededBy: meta.SupersededBy,
	}
	return &entry, deprecationIssues(dep, entry)
}

// deprecationIssues surfaces a non-blocking warning when a referenced
// dependency is deprecated (GH#835/#842). K-033: the dep is still
// resolvable — we warn and point to the successor, we never fail the
// scan on it. Returns nil (not an empty slice) when not deprecated, so
// the happy path is unchanged.
func deprecationIssues(dep parse.DependencyRef, entry cacheEntry) []storage.ValidationIssue {
	if !entry.Deprecated {
		return nil
	}
	msg := fmt.Sprintf("dependency %s@%s is deprecated", dep.Name, dep.Version)
	if entry.SupersededBy != nil && *entry.SupersededBy != "" {
		msg += fmt.Sprintf(" — superseded by %s", *entry.SupersededBy)
	}
	return []storage.ValidationIssue{{
		Code:           "dependency.deprecated",
		Severity:       storage.SeverityWarning,
		Message:        msg,
		Field:          "dependencies",
		ReferencedSlug: dep.Name,
		ReferenceKind:  "dependency",
	}}
}

// hubMetadata is the wire shape returned by GET /api/shared/<slug>/<v>/metadata
// (Option A endpoint extension per GH#630 plan approval; node-shape
// fields slug/title/content added per GH#650).
//
// ── Hub API type-drift surface (GH#528) ─────────────────────────────
//
// Four type sources for the Hub HTTP API exist today; none are
// auto-generated. Updates to one MUST be propagated to the others
// manually — there is no canonical schema source yet.
//
//  1. Hub server (canonical de facto) — defines the wire shape.
//  2. engine/hubapi.Skrpt — used by CLI's `hub list/search/import`
//     commands. Lives in skrptiq-app/engine/hubapi/types.go.
//  3. depresolver.hubMetadata + NodeInfo (here) — used by `scan`'s
//     dep resolution. The only Go-side shape for /api/shared/.
//  4. electron-app/src/main/hub/api-client.ts:HubSkrpt + HubSharedMetadata
//     — TS client used by the desktop app.
//
// Live drift-detection tests (`SKRPTIQ_HUB_URL`-gated) live at:
//
//	internal/engine/hub_drift_test.go — pins engine/hubapi.Skrpt
//	internal/scan/depresolver/live_test.go — pins this hubMetadata shape
//
// CI skips them by default; run locally or in coordinated cross-repo
// verification when touching any of the four sources above. The
// architectural reform (single canonical schema source) is tracked in
// GH#528 and deferred until #525-#527 land.
type hubMetadata struct {
	ID       string     `json:"id"`
	Version  string     `json:"version"`
	Checksum string     `json:"checksum"`
	Nodes    []NodeInfo `json:"nodes"`
	// GH#835/#842 — discovery-only deprecation flag + successor pointer,
	// additive on the resolution response. A deprecated dep is still
	// resolvable (K-033); scan surfaces a non-blocking warning, never a
	// hard failure. SupersededBy is nil when absent/null.
	Deprecated   bool    `json:"deprecated,omitempty"`
	SupersededBy *string `json:"supersededBy,omitempty"`
}

// NodeInfo is one entry in the dep's node list. Wire shape mirrors
// engine.storage.DepNodeSummary so the validator (GH#650
// ValidateWorkflowWithDeps) can consume it after a trivial mapping.
//
//   - ID is the catalogue UUID (K-037 canonical identity).
//   - Slug is the logical slug (= FileSlug on the synthetic node).
//   - Type is the node type ("skill", "prompt", "service", …).
//   - Title is the display title; seeds the validator's priorTitles
//     for plumbing/binding from_step checks.
//   - Content is the node body; required only for prompt nodes that a
//     for_each loop body resolves through {{loop.item}}.
type NodeInfo struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
}

func (r *Resolver) fetchMetadata(dep parse.DependencyRef) (*hubMetadata, error) {
	if dep.Name == "" {
		return nil, fmt.Errorf("dependency %q has empty name field — K-037 requires logical slug", dep.ID)
	}
	url := fmt.Sprintf("%s/api/shared/%s/%s/metadata", r.hubBaseURL, dep.Name, dep.Version)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var meta hubMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}
	if meta.Checksum == "" {
		return nil, fmt.Errorf("metadata missing checksum field")
	}
	return &meta, nil
}

// --- cache ---

// currentCacheSchemaVersion is the on-disk version of the dep-nodes
// cache file format. Bump this whenever cacheEntry, cacheFile, or
// any nested type (NodeInfo, hubMetadata, etc.) changes shape in a
// way that would silently misinterpret an older payload.
//
// GH#654 motivation: before this constant existed, a cache populated
// with the pre-GH#650 NodeInfo (id + type only) would survive an
// upgrade to v0.0.19's NodeInfo (id + slug + title + content) but
// deserialise with empty Slug fields. The validator's depNodes-merge
// then silently skipped those entries (engine guard:
// `if dep.Slug == "" { continue }`), surfacing as false-positive
// `dependency.unresolved_slug` errors that only `rm ~/.skrptiq/cache/`
// could fix.
//
// Schema version 1 corresponds to v0.0.19's GH#650 NodeInfo shape
// (id, slug, type, title, content?). v0.0.16–v0.0.18 wrote no
// version field; readCache treats that as v0 and drops the file.
//
// GH#722 bump (1 → 2): the cache key is the *declared* id@version@checksum
// and a cache hit returns before fetchMetadata + the new id/version
// assertion ever run (resolveDep). A dep cached under the old
// checksum-only code would therefore be served on the next scan without
// the §6.2c identity gate firing, leaving the hole open for already-cached
// deps until natural expiry. A v1 entry is a checksum-keyed acceptance —
// not trustworthy under the stricter id+version+checksum gate — so it must
// be dropped and re-resolved under the full triple. Bumping invalidates
// every pre-fix cache file via the readCache schema-version gate.
const currentCacheSchemaVersion = 2

type cacheEntry struct {
	ID        string     `json:"id"`
	Version   string     `json:"version"`
	Checksum  string     `json:"checksum"`
	Nodes     []NodeInfo `json:"nodes"`
	FetchedAt string     `json:"fetched_at"`
	// Deprecation carried in the cache so a cache HIT warns identically to a
	// fresh fetch (GH#842). Old cache entries lack these → zero value =
	// not deprecated, which is correct.
	Deprecated   bool    `json:"deprecated,omitempty"`
	SupersededBy *string `json:"superseded_by,omitempty"`
}

type cacheFile struct {
	// SchemaVersion is checked on read; a mismatch (including absent /
	// zero, which is what pre-GH#654 cache files have) causes the
	// reader to return an empty entry map without surfacing an error.
	// The next writeCache rewrites the file at the current version.
	SchemaVersion int                   `json:"schemaVersion"`
	Entries       map[string]cacheEntry `json:"entries"`
}

func cacheKey(dep parse.DependencyRef) string {
	return dep.ID + "@" + dep.Version + "@" + dep.Checksum
}

func (r *Resolver) cachePath() string {
	return filepath.Join(r.cacheDir, "dep-nodes.json")
}

func (r *Resolver) readCache() (map[string]cacheEntry, error) {
	data, err := os.ReadFile(r.cachePath())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]cacheEntry{}, nil
		}
		return map[string]cacheEntry{}, err
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return map[string]cacheEntry{}, err
	}
	// Schema-version gate (GH#654). Mismatch means the on-disk shape
	// can't be trusted to match what cacheEntry/NodeInfo currently
	// model — treat as a miss and let writeCache replace the file at
	// the current version on the next merge.
	if cf.SchemaVersion != currentCacheSchemaVersion {
		return map[string]cacheEntry{}, nil
	}
	if cf.Entries == nil {
		cf.Entries = map[string]cacheEntry{}
	}
	return cf.Entries, nil
}

func (r *Resolver) writeCache(entries map[string]cacheEntry) error {
	if err := os.MkdirAll(r.cacheDir, 0o755); err != nil {
		return err
	}

	lockPath := r.cachePath() + ".lock"
	lk := flock.New(lockPath)
	if err := lk.Lock(); err != nil {
		return fmt.Errorf("acquire cache lock: %w", err)
	}
	defer lk.Unlock()

	// Merge with the on-disk state under lock to avoid clobbering a
	// concurrent scan's freshly-fetched entries.
	merged, _ := r.readCache()
	for k, v := range entries {
		merged[k] = v
	}

	data, err := json.MarshalIndent(cacheFile{
		SchemaVersion: currentCacheSchemaVersion,
		Entries:       merged,
	}, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.cachePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.cachePath())
}
