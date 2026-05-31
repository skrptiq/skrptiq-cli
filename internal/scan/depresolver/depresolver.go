// Package depresolver resolves the node-slug membership of a skrpt's
// declared dependencies for scanner-time workflow-execution validation.
//
// Background: GH#630. Under the dep-referenced model (GH#612), a consumer
// bundle's `workflow.execution[].skill` / `.prompt` refs may resolve to
// nodes that live in a declared dep (`hub-shared/<slug>@<v>`) rather than
// the local package. The scanner needs to know which slugs each dep
// provides so the three-tier resolution (local → dep-provided → ERROR)
// can fire.
//
// Trust chain (plan §3): manifest `DependencyRef.Checksum` is the trust
// anchor. The resolver fetches metadata from Hub, verifies the returned
// checksum equals the manifest-declared checksum, then accepts the
// returned node list. A mismatch is a hard ERROR — closes the CTO §F2.1
// phantom-dependency hole. There is no fallback path; if verification
// fails the scan fails.
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

// hubIDPrefix is the only catalogue namespace v1 supports. Plan §1:
// dep IDs are `hub-shared/<slug>`.
const hubIDPrefix = "hub-shared/"

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
	// ProvidedSlugs maps slug → the dep ID providing it (e.g.
	// "llm-service" → "hub-shared/llm-service"). If two deps provide the
	// same slug the last one wins; this is currently not a configured
	// failure mode (multi-dep collision is rare given the convention
	// one-bundle-one-slug); future contract amendment may tighten.
	ProvidedSlugs map[string]string
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
			res.ProvidedSlugs[n.ID] = dep.ID
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
	if !strings.HasPrefix(dep.ID, hubIDPrefix) {
		return nil, []storage.ValidationIssue{{
			Code:     "dependency.unsupported_id_prefix",
			Severity: storage.SeverityError,
			Message:  fmt.Sprintf("dependency %q: only %q prefix supported in v1", dep.ID, hubIDPrefix),
			Field:    "dependencies",
		}}
	}

	if entry, hit := cache[cacheKey(dep)]; hit {
		return &entry, nil
	}

	meta, err := r.fetchMetadata(dep)
	if err != nil {
		return nil, []storage.ValidationIssue{{
			Code:     "dependency.fetch_failed",
			Severity: storage.SeverityError,
			Message:  fmt.Sprintf("dependency %s@%s: %v", dep.ID, dep.Version, err),
			Field:    "dependencies",
		}}
	}

	if meta.Checksum != dep.Checksum {
		return nil, []storage.ValidationIssue{{
			Code:     "dependency.checksum_mismatch",
			Severity: storage.SeverityError,
			Message:  fmt.Sprintf("dependency %s@%s: manifest checksum %s != Hub-reported %s", dep.ID, dep.Version, dep.Checksum, meta.Checksum),
			Field:    "dependencies",
		}}
	}

	entry := cacheEntry{
		ID:        dep.ID,
		Version:   dep.Version,
		Checksum:  dep.Checksum,
		Nodes:     meta.Nodes,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return &entry, nil
}

// hubMetadata is the wire shape returned by GET /api/shared/<slug>/<v>/metadata
// (Option A endpoint extension per GH#630 plan approval).
type hubMetadata struct {
	ID       string     `json:"id"`
	Version  string     `json:"version"`
	Checksum string     `json:"checksum"`
	Nodes    []NodeInfo `json:"nodes"`
}

// NodeInfo is one entry in the dep's node list. Type is informational
// for v1 (scanner doesn't yet distinguish skill-vs-prompt resolution by
// dep-provided type).
type NodeInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (r *Resolver) fetchMetadata(dep parse.DependencyRef) (*hubMetadata, error) {
	slug := strings.TrimPrefix(dep.ID, hubIDPrefix)
	if slug == "" {
		return nil, fmt.Errorf("empty slug in dependency id %q", dep.ID)
	}
	url := fmt.Sprintf("%s/api/shared/%s/%s/metadata", r.hubBaseURL, slug, dep.Version)
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

type cacheEntry struct {
	ID        string     `json:"id"`
	Version   string     `json:"version"`
	Checksum  string     `json:"checksum"`
	Nodes     []NodeInfo `json:"nodes"`
	FetchedAt string     `json:"fetched_at"`
}

type cacheFile struct {
	Entries map[string]cacheEntry `json:"entries"`
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

	data, err := json.MarshalIndent(cacheFile{Entries: merged}, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.cachePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.cachePath())
}
