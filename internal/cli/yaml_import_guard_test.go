package cli

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDirectYAMLv3Import is the GH#530 Phase 3b regression guard: the
// engine is the single YAML parse authority, so no CLI-owned package may
// import gopkg.in/yaml.v3 directly. The engine module imports it
// internally — that is the whole point — but it lives in a sibling module
// (../skrptiq-app/engine) and is not under this repo's tree, so it is not
// walked here. Without this guard, a duplicate CLI-side YAML decoder could
// silently return.
func TestNoDirectYAMLv3Import(t *testing.T) {
	const banned = "gopkg.in/yaml.v3"
	root := moduleRoot(t)

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || (name != "." && strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			if strings.Trim(imp.Path.Value, `"`) == banned {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking module tree: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("%s must not be imported directly by CLI packages — the engine is the single parse authority (GH#530). Offending files:\n  %s",
			banned, strings.Join(offenders, "\n  "))
	}
}

// moduleRoot ascends from the test's working directory to the directory
// holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found ascending from %s", dir)
		}
		dir = parent
	}
}
