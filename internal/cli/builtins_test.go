package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	engexec "github.com/skrptiq/engine/execution"
)

// runBuiltinsJSON invokes emitBuiltinsJSON and captures stdout.
func runBuiltinsJSON(t *testing.T) []byte {
	t.Helper()
	r, w, _ := os.Pipe()
	orig := os.Stdout
	os.Stdout = w
	code := emitBuiltinsJSON()
	w.Close()
	os.Stdout = orig
	if code != ExitOK {
		t.Fatalf("emitBuiltinsJSON exit %d", code)
	}
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return []byte(sb.String())
}

func TestBuiltinsJSON_ShapeAndCoverage(t *testing.T) {
	out := runBuiltinsJSON(t)

	var doc builtinRegistryDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("emit is not valid JSON: %v", err)
	}
	if doc.Operations == nil {
		t.Fatal("missing top-level `operations` object")
	}
	// Every engine builtin present with the LOCKED fields populated.
	for _, d := range engexec.Builtins() {
		e, ok := doc.Operations[d.Name]
		if !ok {
			t.Errorf("op %q missing from emit", d.Name)
			continue
		}
		if e.Title == "" || e.Category == "" {
			t.Errorf("op %q emitted with empty title/category: %+v", d.Name, e)
		}
		if !e.Offline || e.TokenCost != 0 {
			t.Errorf("op %q must be offline + 0 tokens: %+v", d.Name, e)
		}
	}
	if len(doc.Operations) != len(engexec.Builtins()) {
		t.Errorf("emit has %d ops, engine has %d", len(doc.Operations), len(engexec.Builtins()))
	}
}

// The LOCKED per-entry field set (GH#877): exactly title/description/category/
// offline/tokenCost — op is the key, never a field.
func TestBuiltinsJSON_EntryFieldSet(t *testing.T) {
	out := runBuiltinsJSON(t)
	var raw struct {
		Operations map[string]map[string]json.RawMessage `json:"operations"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	want := []string{"category", "description", "offline", "title", "tokenCost"}
	for op, entry := range raw.Operations {
		got := make([]string, 0, len(entry))
		for k := range entry {
			got = append(got, k)
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("op %q field set %v != locked %v", op, got, want)
		}
		if _, leaked := entry["operation"]; leaked {
			t.Errorf("op %q must not repeat `operation` as a field", op)
		}
		break // shape is uniform; one entry suffices
	}
}

// Deterministic: repeated emits are byte-identical (sorted keys), so the Hub's
// `git diff --exit-code` regen check is stable.
func TestBuiltinsJSON_Deterministic(t *testing.T) {
	a := runBuiltinsJSON(t)
	b := runBuiltinsJSON(t)
	if string(a) != string(b) {
		t.Error("emit is not byte-stable across runs")
	}
	// keys must be in sorted order in the raw text
	var doc builtinRegistryDoc
	_ = json.Unmarshal(a, &doc)
	keys := make([]string, 0, len(doc.Operations))
	for k := range doc.Operations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		idx := strings.Index(string(a), "\""+k+"\":")
		if i > 0 {
			prev := strings.Index(string(a), "\""+keys[i-1]+"\":")
			if idx < prev {
				t.Errorf("keys not emitted in sorted order (%q before %q)", k, keys[i-1])
			}
		}
	}
}

// End-to-end: the built binary's `builtins --json` runs engine-free (no DB) and
// emits valid JSON. Guards the subcommand wiring + the no-DB path.
func TestBuiltinsSubcommand_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skips go build in -short")
	}
	bin := t.TempDir() + "/skrptiq"
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = repoRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "builtins", "--json").Output()
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	var doc builtinRegistryDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("subcommand emit not valid JSON: %v", err)
	}
	if len(doc.Operations) == 0 {
		t.Error("subcommand emitted no operations")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// internal/cli -> repo root is two levels up.
	wd, _ := os.Getwd()
	return wd + "/../.."
}
