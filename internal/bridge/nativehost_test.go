package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildNativeHostManifest(t *testing.T) {
	m := buildNativeHostManifest("/some/wrapper")
	if m.Name != NativeHostName {
		t.Errorf("name = %q, want %q", m.Name, NativeHostName)
	}
	if m.Type != "stdio" {
		t.Errorf("type = %q, want stdio", m.Type)
	}
	if m.Path != "/some/wrapper" {
		t.Errorf("path = %q", m.Path)
	}
	want := "chrome-extension://" + PinnedExtensionID + "/"
	if len(m.AllowedOrigins) != 1 || m.AllowedOrigins[0] != want {
		t.Errorf("allowed_origins = %v, want [%q]", m.AllowedOrigins, want)
	}
}

func TestNativeHostWrapperContent(t *testing.T) {
	w := nativeHostWrapperContent("/opt/skrptiq")
	if !strings.HasPrefix(w, "#!/bin/sh") {
		t.Error("wrapper must be a shell script")
	}
	if !strings.Contains(w, "__bridge --mode host") {
		t.Errorf("wrapper must exec host mode; got:\n%s", w)
	}
	if !strings.Contains(w, "/opt/skrptiq") {
		t.Error("wrapper must reference the CLI binary path")
	}
}

func TestInstallUninstallRoundTrip(t *testing.T) {
	nmDir := t.TempDir()
	wrapDir := t.TempDir()
	opts := nativeHostOpts{nmDir: nmDir, wrapperDir: wrapDir, cliBinary: "/opt/skrptiq"}

	if isNativeHostInstalled(opts) {
		t.Fatal("should not be installed before install")
	}

	paths, err := installNativeHost(opts)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !isNativeHostInstalled(opts) {
		t.Error("should report installed after install")
	}

	// Manifest is valid JSON with the pinned origin + wrapper path.
	raw, err := os.ReadFile(paths.manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m nativeHostManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest not valid JSON: %v", err)
	}
	if m.Path != paths.wrapperPath {
		t.Errorf("manifest path %q != wrapper %q", m.Path, paths.wrapperPath)
	}
	if filepath.Dir(paths.manifestPath) != nmDir {
		t.Errorf("manifest not written into nmDir")
	}

	// Wrapper is executable (0700) and references the binary.
	info, err := os.Stat(paths.wrapperPath)
	if err != nil {
		t.Fatalf("stat wrapper: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("wrapper perms = %o, want 0700", info.Mode().Perm())
	}

	uninstallNativeHost(opts)
	if isNativeHostInstalled(opts) {
		t.Error("should not report installed after uninstall")
	}
}
