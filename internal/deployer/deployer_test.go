package deployer

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/slauger/openvox-code/internal/resolver"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestChangeString(t *testing.T) {
	tests := []struct {
		name   string
		change Change
		want   string
	}{
		{
			name:   "add environment",
			change: Change{Type: "add", Environment: "production"},
			want:   "+ production",
		},
		{
			name:   "add module",
			change: Change{Type: "add", Environment: "production", Module: "stdlib", NewRef: "v9.0.0"},
			want:   "+ production/stdlib (v9.0.0)",
		},
		{
			name:   "remove environment",
			change: Change{Type: "remove", Environment: "old-env"},
			want:   "- old-env",
		},
		{
			name:   "remove module",
			change: Change{Type: "remove", Environment: "prod", Module: "deprecated"},
			want:   "- prod/deprecated",
		},
		{
			name:   "update module",
			change: Change{Type: "update", Environment: "prod", Module: "stdlib", OldRef: "v8.0", NewRef: "v9.0"},
			want:   "~ prod/stdlib (v8.0 -> v9.0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.change.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiffNewEnvironment(t *testing.T) {
	dir := t.TempDir()
	d := New(dir, nil, testLogger())

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "production",
			Ref:  "main",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: "https://example.com/stdlib.git", Ref: "v9.0.0"},
			},
		},
	}

	changes, err := d.Diff(envs)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if len(changes) != 2 {
		t.Fatalf("got %d changes, want 2 (env + module)", len(changes))
	}

	if changes[0].Type != "add" || changes[0].Environment != "production" {
		t.Errorf("changes[0] = %v, want add production", changes[0])
	}
}

func TestDiffRemoveEnvironment(t *testing.T) {
	dir := t.TempDir()

	// Create an existing environment
	if err := os.MkdirAll(filepath.Join(dir, "stale-env", "modules"), 0o750); err != nil {
		t.Fatal(err)
	}

	d := New(dir, nil, testLogger())

	// No environments expected
	changes, err := d.Diff(nil)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(changes))
	}
	if changes[0].Type != "remove" || changes[0].Environment != "stale-env" {
		t.Errorf("changes[0] = %v, want remove stale-env", changes[0])
	}
}

func TestDiffRemoveModule(t *testing.T) {
	dir := t.TempDir()

	// Create existing environment with a module
	envDir := filepath.Join(dir, "prod", "modules", "old-module")
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		t.Fatal(err)
	}

	d := New(dir, nil, testLogger())

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "prod",
			Ref:  "main",
			Modules: []resolver.ResolvedModule{
				{Name: "new-module", GitURL: "https://example.com/new.git", Ref: "v1"},
			},
		},
	}

	changes, err := d.Diff(envs)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	hasAdd := false
	hasRemove := false
	for _, c := range changes {
		if c.Type == "add" && c.Module == "new-module" {
			hasAdd = true
		}
		if c.Type == "remove" && c.Module == "old-module" {
			hasRemove = true
		}
	}

	if !hasAdd {
		t.Error("expected add for new-module")
	}
	if !hasRemove {
		t.Error("expected remove for old-module")
	}
}

func TestCleanStale(t *testing.T) {
	dir := t.TempDir()

	// Create some environments
	for _, name := range []string{"keep", "stale1", "stale2"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}

	d := New(dir, nil, testLogger())

	deployed := map[string]bool{"keep": true}
	if err := d.cleanStale(deployed); err != nil {
		t.Fatalf("cleanStale() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 remaining dir, got %d", len(entries))
	}
	if entries[0].Name() != "keep" {
		t.Errorf("remaining dir = %q, want %q", entries[0].Name(), "keep")
	}
}

func TestListExistingEnvironments(t *testing.T) {
	dir := t.TempDir()

	// Create some dirs including hidden and tmp ones
	for _, name := range []string{"production", "staging", ".hidden", "temp.tmp"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	// Create a regular file (should be ignored)
	if err := os.WriteFile(filepath.Join(dir, "somefile"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := New(dir, nil, testLogger())
	envs, err := d.listExistingEnvironments()
	if err != nil {
		t.Fatalf("listExistingEnvironments() error = %v", err)
	}

	if len(envs) != 2 {
		t.Fatalf("got %d environments, want 2", len(envs))
	}
	if envs[0] != "production" || envs[1] != "staging" {
		t.Errorf("environments = %v, want [production staging]", envs)
	}
}

func TestListExistingEnvironmentsEmpty(t *testing.T) {
	d := New("/nonexistent/path", nil, testLogger())
	envs, err := d.listExistingEnvironments()
	if err != nil {
		t.Fatalf("listExistingEnvironments() error = %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("got %d environments, want 0", len(envs))
	}
}
