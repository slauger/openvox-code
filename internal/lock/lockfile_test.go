package lock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openvox-code.lock")

	lf := NewFromResolved(map[string]LockedEnvironment{
		"production": {
			Ref: "v1.5.0",
			Modules: []LockedModule{
				{Name: "stdlib", Git: "https://github.com/puppetlabs/stdlib.git", Ref: "v9.0.0", SHA: "abc123def456"},
				{Name: "apache", Git: "https://github.com/puppetlabs/apache.git", Ref: "v12.0.0", SHA: "789abc012def"},
			},
		},
		"staging": {
			Ref:            "staging",
			ControlRepoURL: "https://github.com/example/control.git",
			ControlRepoSHA: "aaa111bbb222",
			Modules: []LockedModule{
				{Name: "stdlib", Git: "https://github.com/puppetlabs/stdlib.git", Ref: "v9.0.0", SHA: "abc123def456"},
			},
		},
	})

	if err := lf.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lockfile not created: %v", err)
	}

	// Load it back
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
	if len(loaded.Environments) != 2 {
		t.Fatalf("Environments length = %d, want 2", len(loaded.Environments))
	}

	prod := loaded.Environments["production"]
	if prod.Ref != "v1.5.0" {
		t.Errorf("production ref = %q, want %q", prod.Ref, "v1.5.0")
	}
	if len(prod.Modules) != 2 {
		t.Fatalf("production modules = %d, want 2", len(prod.Modules))
	}
	// Modules should be sorted by name
	if prod.Modules[0].Name != "apache" {
		t.Errorf("first module = %q, want %q (sorted)", prod.Modules[0].Name, "apache")
	}
	if prod.Modules[1].Name != "stdlib" {
		t.Errorf("second module = %q, want %q (sorted)", prod.Modules[1].Name, "stdlib")
	}

	staging := loaded.Environments["staging"]
	if staging.ControlRepoURL != "https://github.com/example/control.git" {
		t.Errorf("staging control URL = %q", staging.ControlRepoURL)
	}
	if staging.ControlRepoSHA != "aaa111bbb222" {
		t.Errorf("staging control SHA = %q", staging.ControlRepoSHA)
	}
}

func TestLoadNotFound(t *testing.T) {
	_, err := Load("/nonexistent/lockfile.lock")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.lock")
	os.WriteFile(path, []byte("{{invalid"), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v99.lock")
	os.WriteFile(path, []byte("version: 99\nenvironments: {}"), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported lockfile version") {
		t.Errorf("error = %q, want mention of unsupported version", err.Error())
	}
}

func TestSaveDeterministic(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "lock1")
	path2 := filepath.Join(dir, "lock2")

	envs := map[string]LockedEnvironment{
		"zebra": {
			Ref: "main",
			Modules: []LockedModule{
				{Name: "z-mod", Git: "https://example.com/z.git", Ref: "v1", SHA: "aaa"},
				{Name: "a-mod", Git: "https://example.com/a.git", Ref: "v2", SHA: "bbb"},
			},
		},
		"alpha": {
			Ref: "main",
			Modules: []LockedModule{
				{Name: "mod-b", Git: "https://example.com/b.git", Ref: "v1", SHA: "ccc"},
				{Name: "mod-a", Git: "https://example.com/a.git", Ref: "v1", SHA: "ddd"},
			},
		},
	}

	lf1 := NewFromResolved(envs)
	lf1.Save(path1)

	// Create again and save
	lf2 := NewFromResolved(envs)
	lf2.Save(path2)

	data1, _ := os.ReadFile(path1)
	data2, _ := os.ReadFile(path2)

	// Remove the generated_at line (timestamps differ)
	lines1 := filterLines(string(data1), "generated_at")
	lines2 := filterLines(string(data2), "generated_at")

	if lines1 != lines2 {
		t.Errorf("lockfile output is not deterministic:\n%s\nvs\n%s", lines1, lines2)
	}
}

func TestNewFromResolved(t *testing.T) {
	envs := map[string]LockedEnvironment{
		"prod": {Ref: "main"},
	}
	lf := NewFromResolved(envs)
	if lf.Version != 1 {
		t.Errorf("Version = %d, want 1", lf.Version)
	}
	if len(lf.Environments) != 1 {
		t.Errorf("Environments length = %d, want 1", len(lf.Environments))
	}
}

func filterLines(s, exclude string) string {
	var result []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.Contains(line, exclude) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}
