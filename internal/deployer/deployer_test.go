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
	d := New(dir, nil, 4, testLogger())

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

	d := New(dir, nil, 4, testLogger())

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

	d := New(dir, nil, 4, testLogger())

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

	d := New(dir, nil, 4, testLogger())

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

	d := New(dir, nil, 4, testLogger())
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

func TestChangeStringUnknownType(t *testing.T) {
	c := Change{Type: "unknown", Environment: "prod", Module: "mod"}
	got := c.String()
	if got != "? prod/mod" {
		t.Errorf("String() = %q, want %q", got, "? prod/mod")
	}
}

func TestDiffSourceDiscoverySkipped(t *testing.T) {
	dir := t.TempDir()
	d := New(dir, nil, 4, testLogger())

	envs := []resolver.ResolvedEnvironment{
		{Name: "__source_discovery__", Ref: "__all__"},
	}

	changes, err := d.Diff(envs)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("expected no changes for source discovery, got %d", len(changes))
	}
}

func TestListExistingModulesEmpty(t *testing.T) {
	d := New("/nonexistent", nil, 4, testLogger())
	mods, err := d.listExistingModules("nonexistent-env")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if mods != nil {
		t.Errorf("expected nil for nonexistent env, got %v", mods)
	}
}

func TestListExistingModules(t *testing.T) {
	dir := t.TempDir()
	modulesDir := filepath.Join(dir, "prod", "modules")
	for _, mod := range []string{"stdlib", "apache"} {
		if err := os.MkdirAll(filepath.Join(modulesDir, mod), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	// Also create a file (should be ignored)
	if err := os.WriteFile(filepath.Join(modulesDir, "README"), []byte("ignore"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := New(dir, nil, 4, testLogger())
	mods, err := d.listExistingModules("prod")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(mods) != 2 {
		t.Errorf("got %d modules, want 2", len(mods))
	}
	if !mods["stdlib"] || !mods["apache"] {
		t.Errorf("modules = %v, want stdlib and apache", mods)
	}
}

func TestMergeModules(t *testing.T) {
	tests := []struct {
		name      string
		global    []resolver.ResolvedModule
		local     []resolver.ResolvedModule
		exclude   map[string]bool
		wantNames map[string]bool
	}{
		{
			name: "local overrides global",
			global: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: "https://example.com/stdlib.git", Ref: "v1"},
			},
			local: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: "https://fork.com/stdlib.git", Ref: "v2"},
			},
			exclude:   nil,
			wantNames: map[string]bool{"stdlib": true},
		},
		{
			name: "exclude removes global",
			global: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: "https://example.com/stdlib.git", Ref: "v1"},
				{Name: "apache", GitURL: "https://example.com/apache.git", Ref: "v2"},
			},
			local:     nil,
			exclude:   map[string]bool{"stdlib": true},
			wantNames: map[string]bool{"apache": true},
		},
		{
			name: "local adds new module",
			global: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: "https://example.com/stdlib.git", Ref: "v1"},
			},
			local: []resolver.ResolvedModule{
				{Name: "custom", GitURL: "https://example.com/custom.git", Ref: "main"},
			},
			exclude:   nil,
			wantNames: map[string]bool{"stdlib": true, "custom": true},
		},
		{
			name:      "empty inputs",
			global:    nil,
			local:     nil,
			exclude:   nil,
			wantNames: map[string]bool{},
		},
		{
			name: "exclude and local combined",
			global: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: "https://example.com/stdlib.git", Ref: "v1"},
				{Name: "deprecated", GitURL: "https://example.com/dep.git", Ref: "v1"},
			},
			local: []resolver.ResolvedModule{
				{Name: "replacement", GitURL: "https://example.com/rep.git", Ref: "main"},
			},
			exclude:   map[string]bool{"deprecated": true},
			wantNames: map[string]bool{"stdlib": true, "replacement": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeModules(tt.global, tt.local, tt.exclude)
			got := make(map[string]bool)
			for _, m := range result {
				got[m.Name] = true
			}
			if len(got) != len(tt.wantNames) {
				t.Errorf("got %d modules, want %d: %v", len(got), len(tt.wantNames), got)
			}
			for name := range tt.wantNames {
				if !got[name] {
					t.Errorf("missing module %q", name)
				}
			}
		})
	}
}

func TestMergeModulesLocalOverridesRef(t *testing.T) {
	global := []resolver.ResolvedModule{
		{Name: "stdlib", GitURL: "https://example.com/stdlib.git", Ref: "v1"},
	}
	local := []resolver.ResolvedModule{
		{Name: "stdlib", GitURL: "https://fork.com/stdlib.git", Ref: "v2"},
	}

	result := mergeModules(global, local, nil)
	if len(result) != 1 {
		t.Fatalf("got %d modules, want 1", len(result))
	}
	if result[0].GitURL != "https://fork.com/stdlib.git" {
		t.Errorf("GitURL = %q, want fork URL", result[0].GitURL)
	}
	if result[0].Ref != "v2" {
		t.Errorf("Ref = %q, want v2", result[0].Ref)
	}
}

func TestExpandModuleFilePaths(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(dir, "modules.yaml"), []byte("modules: []"), 0o600); err != nil {
		t.Fatal(err)
	}
	extrasDir := filepath.Join(dir, "extras")
	if err := os.MkdirAll(extrasDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extrasDir, "a.yaml"), []byte("modules: []"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extrasDir, "b.yaml"), []byte("modules: []"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := New(dir, nil, 4, testLogger())

	tests := []struct {
		name      string
		pattern   string
		wantCount int
		wantErr   bool
	}{
		{name: "exact path", pattern: "modules.yaml", wantCount: 1},
		{name: "glob pattern", pattern: "extras/*.yaml", wantCount: 2},
		{name: "nonexistent exact", pattern: "missing.yaml", wantErr: true},
		{name: "glob no match", pattern: "nope/*.yaml", wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := d.expandModuleFilePaths(dir, tt.pattern)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.wantCount {
				t.Errorf("got %d paths, want %d: %v", len(paths), tt.wantCount, paths)
			}
		})
	}
}

func TestReadModuleFileK8sStyle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "modules.yaml")

	content := `apiVersion: openvox.voxpupuli.org/v1alpha1
kind: ModuleFile
spec:
  modules:
    - name: stdlib
      git: https://example.com/stdlib.git
      ref: v9.0.0
    - name: apache
      git: https://example.com/apache.git
      follow_branch: true
  exclude:
    - old_module
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	d := New(dir, nil, 4, testLogger())
	parsed, err := d.readModuleFile(path)
	if err != nil {
		t.Fatalf("readModuleFile() error = %v", err)
	}

	if len(parsed.modules) != 2 {
		t.Fatalf("modules len = %d, want 2", len(parsed.modules))
	}
	if parsed.modules[0].Name != "stdlib" || parsed.modules[0].Ref != "v9.0.0" {
		t.Errorf("modules[0] = %+v", parsed.modules[0])
	}
	if !parsed.modules[1].FollowBranch {
		t.Error("modules[1].FollowBranch should be true")
	}
	if !parsed.exclude["old_module"] {
		t.Error("exclude should contain old_module")
	}
}

func TestReadModuleFileFlatFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "modules.yaml")

	content := `modules:
  - name: stdlib
    git: https://example.com/stdlib.git
    ref: v9.0.0
    target_dir: vendor
    install_as: puppetlabs-stdlib
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	d := New(dir, nil, 4, testLogger())
	parsed, err := d.readModuleFile(path)
	if err != nil {
		t.Fatalf("readModuleFile() error = %v", err)
	}

	if len(parsed.modules) != 1 {
		t.Fatalf("modules len = %d, want 1", len(parsed.modules))
	}
	m := parsed.modules[0]
	if m.TargetDir != "vendor" {
		t.Errorf("TargetDir = %q, want vendor", m.TargetDir)
	}
	if m.InstallAs != "puppetlabs-stdlib" {
		t.Errorf("InstallAs = %q, want puppetlabs-stdlib", m.InstallAs)
	}
}

func TestReadModuleFileNotFound(t *testing.T) {
	d := New(t.TempDir(), nil, 4, testLogger())
	_, err := d.readModuleFile("/nonexistent/modules.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestListExistingEnvironmentsEmpty(t *testing.T) {
	d := New("/nonexistent/path", nil, 4, testLogger())
	envs, err := d.listExistingEnvironments()
	if err != nil {
		t.Fatalf("listExistingEnvironments() error = %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("got %d environments, want 0", len(envs))
	}
}
