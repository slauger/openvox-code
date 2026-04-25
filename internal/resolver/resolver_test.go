package resolver

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/slauger/openvox-code/internal/config"
)

// mockBranchLister implements BranchLister for testing.
type mockBranchLister struct {
	branches map[string][]string
}

func (m *mockBranchLister) ListBranches(_ context.Context, gitURL string) ([]string, error) {
	if branches, ok := m.branches[gitURL]; ok {
		return branches, nil
	}
	return nil, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestResolveStaticEnvironments(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
		ModuleSets: map[string][]config.Module{
			"base": {
				{Name: "stdlib", Git: "https://github.com/puppetlabs/puppetlabs-stdlib.git", Ref: "v9.0.0"},
				{Name: "concat", Git: "https://github.com/puppetlabs/puppetlabs-concat.git", Ref: "v9.0.0"},
			},
		},
		Environments: map[string]*config.Environment{
			"production": {
				Ref:        "v1.5.0",
				ModuleSets: []string{"base"},
			},
			"staging": {
				Ref:        "staging",
				ModuleSets: []string{"base"},
				Modules: []config.Module{
					{Name: "custom", Git: "https://github.com/example/custom.git", Ref: "main"},
				},
			},
		},
	}

	r := New(cfg, testLogger())
	envs, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(envs) != 2 {
		t.Fatalf("got %d environments, want 2", len(envs))
	}

	envMap := make(map[string]ResolvedEnvironment)
	for _, env := range envs {
		envMap[env.Name] = env
	}

	prod, ok := envMap["production"]
	if !ok {
		t.Fatal("production environment not found")
	}
	if prod.Ref != "v1.5.0" {
		t.Errorf("production ref = %q, want %q", prod.Ref, "v1.5.0")
	}
	if len(prod.Modules) != 2 {
		t.Errorf("production modules = %d, want 2", len(prod.Modules))
	}

	staging, ok := envMap["staging"]
	if !ok {
		t.Fatal("staging environment not found")
	}
	if len(staging.Modules) != 3 {
		t.Errorf("staging modules = %d, want 3", len(staging.Modules))
	}
}

func TestResolveModuleOverride(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
		ModuleSets: map[string][]config.Module{
			"base": {
				{Name: "stdlib", Git: "https://github.com/puppetlabs/puppetlabs-stdlib.git", Ref: "v9.0.0"},
			},
		},
		Environments: map[string]*config.Environment{
			"dev": {
				Ref:        "develop",
				ModuleSets: []string{"base"},
				Modules: []config.Module{
					{Name: "stdlib", Git: "https://github.com/my-fork/stdlib.git", Ref: "my-branch"},
				},
			},
		},
	}

	r := New(cfg, testLogger())
	envs, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(envs) != 1 {
		t.Fatalf("got %d environments, want 1", len(envs))
	}

	// Inline module should override moduleset module
	if len(envs[0].Modules) != 1 {
		t.Fatalf("got %d modules, want 1", len(envs[0].Modules))
	}

	mod := envs[0].Modules[0]
	if mod.GitURL != "https://github.com/my-fork/stdlib.git" {
		t.Errorf("module git = %q, want fork URL", mod.GitURL)
	}
	if mod.Ref != "my-branch" {
		t.Errorf("module ref = %q, want %q", mod.Ref, "my-branch")
	}
}

func TestResolveWithGitMirror(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
		Overrides:      config.Overrides{GitMirror: "https://mirror.example.com"},
		ModuleSets: map[string][]config.Module{
			"base": {
				{Name: "stdlib", Git: "https://github.com/puppetlabs/puppetlabs-stdlib.git", Ref: "v9.0.0"},
			},
		},
		Environments: map[string]*config.Environment{
			"prod": {Ref: "main", ModuleSets: []string{"base"}},
		},
	}

	r := New(cfg, testLogger())
	envs, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(envs[0].Modules) != 1 {
		t.Fatalf("got %d modules, want 1", len(envs[0].Modules))
	}

	got := envs[0].Modules[0].GitURL
	want := "https://mirror.example.com/puppetlabs/puppetlabs-stdlib.git"
	if got != want {
		t.Errorf("module URL = %q, want %q (mirrored)", got, want)
	}
}

func TestResolveSourceBranches(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
		Sources: []config.Source{
			{
				URL:            "https://github.com/example/control-repo.git",
				BranchSelector: config.BranchSelector{MatchPatterns: []string{"production", "staging"}},
			},
		},
	}

	r := New(cfg, testLogger())
	envs, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(envs) != 2 {
		t.Fatalf("got %d environments, want 2", len(envs))
	}

	names := make(map[string]bool)
	for _, env := range envs {
		names[env.Name] = true
	}
	if !names["production"] || !names["staging"] {
		t.Errorf("expected production and staging, got %v", names)
	}
}

func TestExpandDiscovery(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
	}
	r := New(cfg, testLogger())

	envs := []ResolvedEnvironment{
		{Name: "production", Ref: "main"},
		{Name: "__source_discovery__", ControlRepoURL: "https://example.com/control.git", Ref: "__all__"},
		{Name: "staging", Ref: "staging"},
	}

	lister := &mockBranchLister{
		branches: map[string][]string{
			"https://example.com/control.git": {"main", "develop", "feature-x"},
		},
	}

	result, err := r.ExpandDiscovery(context.Background(), envs, lister)
	if err != nil {
		t.Fatalf("ExpandDiscovery() error = %v", err)
	}

	// Should have: production + staging (kept) + main + develop + feature-x (discovered)
	if len(result) != 5 {
		t.Fatalf("got %d environments, want 5, got: %v", len(result), envNames(result))
	}

	names := make(map[string]bool)
	for _, env := range result {
		names[env.Name] = true
	}
	for _, expected := range []string{"production", "staging", "main", "develop", "feature-x"} {
		if !names[expected] {
			t.Errorf("missing environment %q in result", expected)
		}
	}

	// Verify discovered envs have control repo URL
	for _, env := range result {
		if env.Name == "develop" || env.Name == "feature-x" || env.Name == "main" {
			if env.ControlRepoURL != "https://example.com/control.git" {
				t.Errorf("discovered env %q missing control repo URL", env.Name)
			}
		}
	}
}

func TestExpandDiscoveryNoop(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
	}
	r := New(cfg, testLogger())

	envs := []ResolvedEnvironment{
		{Name: "production", Ref: "main"},
	}

	result, err := r.ExpandDiscovery(context.Background(), envs, &mockBranchLister{})
	if err != nil {
		t.Fatalf("ExpandDiscovery() error = %v", err)
	}
	if len(result) != 1 || result[0].Name != "production" {
		t.Errorf("expected passthrough, got %v", result)
	}
}

func envNames(envs []ResolvedEnvironment) []string {
	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = e.Name
	}
	return names
}

func TestContainsGlob(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"main", false},
		{"production", false},
		{"feature/*", true},
		{"release-?.x", true},
		{"[abc]", true},
		{"feature/login", false},
		{"*", true},
		{"", false},
		{"refs/heads/main", false},
		{"release-*-hotfix", true},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := containsGlob(tt.pattern)
			if got != tt.want {
				t.Errorf("containsGlob(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestResolveSourceEnvironmentsGlobPatterns(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
		Sources: []config.Source{
			{
				URL: "https://example.com/repo.git",
				BranchSelector: config.BranchSelector{
					MatchPatterns: []string{"feature/*", "release-*"},
				},
			},
		},
	}

	r := New(cfg, testLogger())
	envs, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Glob patterns should produce a discovery placeholder
	if len(envs) != 1 {
		t.Fatalf("got %d environments, want 1 (discovery placeholder)", len(envs))
	}
	if envs[0].Name != "__source_discovery__" {
		t.Errorf("Name = %q, want __source_discovery__", envs[0].Name)
	}
	if envs[0].Ref != "__patterns__" {
		t.Errorf("Ref = %q, want __patterns__", envs[0].Ref)
	}
	if envs[0].BranchSelector == nil {
		t.Fatal("BranchSelector should not be nil")
	}
}

func TestResolveSourceEnvironmentsMatchesAll(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
		Sources: []config.Source{
			{
				URL:            "https://example.com/repo.git",
				BranchSelector: config.BranchSelector{},
			},
		},
	}

	r := New(cfg, testLogger())
	envs, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(envs) != 1 {
		t.Fatalf("got %d environments, want 1", len(envs))
	}
	if envs[0].Name != "__source_discovery__" {
		t.Errorf("Name = %q, want __source_discovery__", envs[0].Name)
	}
	if envs[0].Ref != "__all__" {
		t.Errorf("Ref = %q, want __all__", envs[0].Ref)
	}
}

func TestResolveSourceEnvironmentsWithModuleFiles(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
		Sources: []config.Source{
			{
				URL:            "https://example.com/repo.git",
				BranchSelector: config.BranchSelector{MatchPatterns: []string{"main"}},
				ModuleFiles: []config.ModuleFileRef{
					{Name: "modules.yaml", Required: true},
				},
			},
		},
	}

	r := New(cfg, testLogger())
	envs, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(envs) != 1 {
		t.Fatalf("got %d environments, want 1", len(envs))
	}
	if len(envs[0].ModuleFiles) != 1 {
		t.Fatalf("ModuleFiles len = %d, want 1", len(envs[0].ModuleFiles))
	}
	if envs[0].ModuleFiles[0].Name != "modules.yaml" {
		t.Errorf("ModuleFile name = %q, want modules.yaml", envs[0].ModuleFiles[0].Name)
	}
}

func TestExpandDiscoveryWithExcludePatterns(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
	}
	r := New(cfg, testLogger())

	selector := config.BranchSelector{
		ExcludePatterns: []string{"wip-*", "test-*"},
	}

	envs := []ResolvedEnvironment{
		{
			Name:           "__source_discovery__",
			ControlRepoURL: "https://example.com/control.git",
			Ref:            "__all__",
			BranchSelector: &selector,
		},
	}

	lister := &mockBranchLister{
		branches: map[string][]string{
			"https://example.com/control.git": {"main", "wip-broken", "feature-x", "test-ci"},
		},
	}

	result, err := r.ExpandDiscovery(context.Background(), envs, lister)
	if err != nil {
		t.Fatalf("ExpandDiscovery() error = %v", err)
	}

	// wip-broken and test-ci should be excluded
	if len(result) != 2 {
		t.Fatalf("got %d environments, want 2, got: %v", len(result), envNames(result))
	}

	names := make(map[string]bool)
	for _, env := range result {
		names[env.Name] = true
	}
	if !names["main"] || !names["feature-x"] {
		t.Errorf("expected main and feature-x, got %v", names)
	}
	if names["wip-broken"] || names["test-ci"] {
		t.Error("excluded branches should not appear")
	}
}

func TestExpandDiscoveryWithMatchAndExclude(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
	}
	r := New(cfg, testLogger())

	selector := config.BranchSelector{
		MatchPatterns:   []string{"feature-*"},
		ExcludePatterns: []string{"feature-wip-*"},
	}

	envs := []ResolvedEnvironment{
		{
			Name:           "__source_discovery__",
			ControlRepoURL: "https://example.com/control.git",
			Ref:            "__patterns__",
			BranchSelector: &selector,
		},
	}

	lister := &mockBranchLister{
		branches: map[string][]string{
			"https://example.com/control.git": {"main", "feature-login", "feature-wip-broken", "bugfix-1"},
		},
	}

	result, err := r.ExpandDiscovery(context.Background(), envs, lister)
	if err != nil {
		t.Fatalf("ExpandDiscovery() error = %v", err)
	}

	// Only feature-login should match (feature-* minus feature-wip-*)
	if len(result) != 1 {
		t.Fatalf("got %d environments, want 1, got: %v", len(result), envNames(result))
	}
	if result[0].Name != "feature-login" {
		t.Errorf("Name = %q, want feature-login", result[0].Name)
	}
}

func TestExpandDiscoveryPreservesModuleFiles(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
	}
	r := New(cfg, testLogger())

	mfs := []config.ModuleFileRef{
		{Name: "modules.yaml", Required: true},
	}

	envs := []ResolvedEnvironment{
		{
			Name:           "__source_discovery__",
			ControlRepoURL: "https://example.com/control.git",
			Ref:            "__all__",
			ModuleFiles:    mfs,
		},
	}

	lister := &mockBranchLister{
		branches: map[string][]string{
			"https://example.com/control.git": {"main"},
		},
	}

	result, err := r.ExpandDiscovery(context.Background(), envs, lister)
	if err != nil {
		t.Fatalf("ExpandDiscovery() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("got %d environments, want 1", len(result))
	}
	if len(result[0].ModuleFiles) != 1 {
		t.Fatalf("ModuleFiles not preserved: %d", len(result[0].ModuleFiles))
	}
	if result[0].ModuleFiles[0].Name != "modules.yaml" {
		t.Errorf("ModuleFile name = %q, want modules.yaml", result[0].ModuleFiles[0].Name)
	}
}

func TestResolvedModuleInstallPath(t *testing.T) {
	tests := []struct {
		name string
		mod  ResolvedModule
		want string
	}{
		{name: "defaults", mod: ResolvedModule{Name: "stdlib"}, want: "modules/stdlib"},
		{name: "custom target_dir", mod: ResolvedModule{Name: "stdlib", TargetDir: "vendor"}, want: "vendor/stdlib"},
		{name: "custom install_as", mod: ResolvedModule{Name: "stdlib", InstallAs: "puppetlabs-stdlib"}, want: "modules/puppetlabs-stdlib"},
		{name: "both custom", mod: ResolvedModule{Name: "stdlib", TargetDir: "vendor", InstallAs: "puppetlabs-stdlib"}, want: "vendor/puppetlabs-stdlib"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mod.InstallPath()
			if got != tt.want {
				t.Errorf("InstallPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateDuplicateModules(t *testing.T) {
	cfg := &config.Config{
		CacheDir:       "/tmp/cache",
		EnvironmentDir: "/tmp/envs",
		ModuleSets: map[string][]config.Module{
			"set1": {{Name: "stdlib", Git: "https://example.com/a.git", Ref: "v1"}},
			"set2": {{Name: "stdlib", Git: "https://example.com/b.git", Ref: "v2"}},
		},
		Environments: map[string]*config.Environment{
			"prod": {Ref: "main", ModuleSets: []string{"set1", "set2"}},
		},
	}

	r := New(cfg, testLogger())
	// With our current implementation, module set modules with the same name
	// will be overwritten (last-writer-wins), so this should NOT error.
	// The duplicate detection in Validate() only checks within a single environment's
	// resolved module list. Since map-based dedup handles it, validate should pass.
	err := r.Validate(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
