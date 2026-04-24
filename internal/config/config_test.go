package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "openvox-code.yaml")

	content := `
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments
  sources:
    - url: https://github.com/example/control-repo.git
      branchSelector: {}
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.CacheDir != "/var/cache/openvox-code" {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, "/var/cache/openvox-code")
	}
	if cfg.EnvironmentDir != "/etc/puppetlabs/code/environments" {
		t.Errorf("EnvironmentDir = %q, want %q", cfg.EnvironmentDir, "/etc/puppetlabs/code/environments")
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("Sources length = %d, want 1", len(cfg.Sources))
	}
	if !cfg.Sources[0].BranchSelector.MatchesAll() {
		t.Error("expected branchSelector to match all (no patterns)")
	}
}

func TestLoadConfigWithEnvironments(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "openvox-code.yaml")

	content := `
cachedir: /tmp/cache
environmentdir: /tmp/envs
modulesets:
  base:
    - name: stdlib
      git: https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: v9.0.0
    - name: concat
      git: https://github.com/puppetlabs/puppetlabs-concat.git
      ref: v9.0.0
environments:
  production:
    ref: v1.5.0
    modulesets:
      - base
  staging:
    ref: staging
    modulesets:
      - base
    modules:
      - name: custom
        git: https://github.com/example/custom.git
        ref: main
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.ModuleSets) != 1 {
		t.Fatalf("ModuleSets length = %d, want 1", len(cfg.ModuleSets))
	}
	if len(cfg.ModuleSets["base"]) != 2 {
		t.Fatalf("ModuleSets[base] length = %d, want 2", len(cfg.ModuleSets["base"]))
	}
	if len(cfg.Environments) != 2 {
		t.Fatalf("Environments length = %d, want 2", len(cfg.Environments))
	}
	if cfg.Environments["staging"].Ref != "staging" {
		t.Errorf("staging ref = %q, want %q", cfg.Environments["staging"].Ref, "staging")
	}
	if len(cfg.Environments["staging"].Modules) != 1 {
		t.Errorf("staging modules = %d, want 1", len(cfg.Environments["staging"].Modules))
	}
}

func TestLoadConfigWithIncludes(t *testing.T) {
	dir := t.TempDir()
	modulesetsDir := filepath.Join(dir, "modulesets")
	if err := os.MkdirAll(modulesetsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	mainCfg := `
cachedir: /tmp/cache
environmentdir: /tmp/envs
includes:
  - modulesets/*.yaml
environments:
  production:
    ref: main
    modulesets:
      - base
`
	baseCfg := `
modulesets:
  base:
    - name: stdlib
      git: https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: v9.0.0
`
	if err := os.WriteFile(filepath.Join(dir, "openvox-code.yaml"), []byte(mainCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modulesetsDir, "base.yaml"), []byte(baseCfg), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "openvox-code.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.ModuleSets) != 1 {
		t.Fatalf("ModuleSets length = %d, want 1", len(cfg.ModuleSets))
	}
	if _, ok := cfg.ModuleSets["base"]; !ok {
		t.Error("ModuleSets[base] not found after include merge")
	}
}

func TestLoadConfigNestedIncludesFails(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0o750); err != nil {
		t.Fatal(err)
	}

	mainCfg := `
cachedir: /tmp/cache
environmentdir: /tmp/envs
includes:
  - sub/nested.yaml
`
	nestedCfg := `
includes:
  - something.yaml
`
	if err := os.WriteFile(filepath.Join(dir, "openvox-code.yaml"), []byte(mainCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.yaml"), []byte(nestedCfg), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "openvox-code.yaml"))
	if err == nil {
		t.Fatal("expected error for nested includes, got nil")
	}
	if !strings.Contains(err.Error(), "nested includes") {
		t.Errorf("error %q should mention nested includes", err.Error())
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "openvox-code.yaml")
	if err := os.WriteFile(cfgFile, []byte("{{invalid yaml"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgFile)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with sources",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
				Sources: []Source{
					{URL: "https://github.com/example/repo.git", BranchSelector: BranchSelector{}},
				},
			},
		},
		{
			name: "valid config with environments",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
				ModuleSets: map[string][]Module{
					"base": {{Name: "stdlib", Git: "https://example.com/stdlib.git", Ref: "v1.0"}},
				},
				Environments: map[string]*Environment{
					"production": {Ref: "main", ModuleSets: []string{"base"}},
				},
			},
		},
		{
			name: "missing cachedir",
			cfg: Config{
				EnvironmentDir: "/tmp/envs",
				Sources:        []Source{{URL: "https://example.com/repo.git", BranchSelector: BranchSelector{}}},
			},
			wantErr: true,
			errMsg:  "cachedir is required",
		},
		{
			name: "missing environmentdir",
			cfg: Config{
				CacheDir: "/tmp/cache",
				Sources:  []Source{{URL: "https://example.com/repo.git", BranchSelector: BranchSelector{}}},
			},
			wantErr: true,
			errMsg:  "environmentdir is required",
		},
		{
			name: "no sources or environments",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
			},
			wantErr: true,
			errMsg:  "at least one source or environment",
		},
		{
			name: "source without url",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
				Sources:        []Source{{BranchSelector: BranchSelector{}}},
			},
			wantErr: true,
			errMsg:  "source url is required",
		},
		{
			name: "source with empty selector matches all",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
				Sources:        []Source{{URL: "https://example.com/repo.git"}},
			},
			wantErr: false,
		},
		{
			name: "environment without ref",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
				Environments:   map[string]*Environment{"prod": {}},
			},
			wantErr: true,
			errMsg:  "ref is required",
		},
		{
			name: "unknown moduleset reference",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
				Environments: map[string]*Environment{
					"prod": {Ref: "main", ModuleSets: []string{"nonexistent"}},
				},
			},
			wantErr: true,
			errMsg:  "unknown moduleset",
		},
		{
			name: "module missing name",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
				ModuleSets: map[string][]Module{
					"base": {{Git: "https://example.com/mod.git", Ref: "v1"}},
				},
				Environments: map[string]*Environment{
					"prod": {Ref: "main", ModuleSets: []string{"base"}},
				},
			},
			wantErr: true,
			errMsg:  "module name is required",
		},
		{
			name: "duplicate module in moduleset",
			cfg: Config{
				CacheDir:       "/tmp/cache",
				EnvironmentDir: "/tmp/envs",
				ModuleSets: map[string][]Module{
					"base": {
						{Name: "stdlib", Git: "https://example.com/a.git", Ref: "v1"},
						{Name: "stdlib", Git: "https://example.com/b.git", Ref: "v2"},
					},
				},
				Environments: map[string]*Environment{
					"prod": {Ref: "main", ModuleSets: []string{"base"}},
				},
			},
			wantErr: true,
			errMsg:  "duplicate module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBranchSelectorMatches(t *testing.T) {
	tests := []struct {
		name     string
		selector BranchSelector
		branch   string
		want     bool
	}{
		{name: "empty matches all", selector: BranchSelector{}, branch: "anything", want: true},
		{name: "exact match", selector: BranchSelector{MatchPatterns: []string{"production"}}, branch: "production", want: true},
		{name: "exact no match", selector: BranchSelector{MatchPatterns: []string{"production"}}, branch: "staging", want: false},
		{name: "glob match", selector: BranchSelector{MatchPatterns: []string{"feature/*"}}, branch: "feature/login", want: true},
		{name: "glob no match", selector: BranchSelector{MatchPatterns: []string{"feature/*"}}, branch: "bugfix/login", want: false},
		{name: "multiple patterns", selector: BranchSelector{MatchPatterns: []string{"production", "staging"}}, branch: "staging", want: true},
		{name: "wildcard", selector: BranchSelector{MatchPatterns: []string{"*"}}, branch: "anything", want: true},
		{name: "exclude", selector: BranchSelector{ExcludePatterns: []string{"wip-*"}}, branch: "wip-broken", want: false},
		{name: "exclude no match", selector: BranchSelector{ExcludePatterns: []string{"wip-*"}}, branch: "production", want: true},
		{name: "match and exclude", selector: BranchSelector{MatchPatterns: []string{"feature/*"}, ExcludePatterns: []string{"feature/wip-*"}}, branch: "feature/login", want: true},
		{name: "match but excluded", selector: BranchSelector{MatchPatterns: []string{"feature/*"}, ExcludePatterns: []string{"feature/wip-*"}}, branch: "feature/wip-broken", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.selector.Matches(tt.branch)
			if got != tt.want {
				t.Errorf("Matches(%q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

func TestBranchSelectorMatchesAll(t *testing.T) {
	s := BranchSelector{}
	if !s.MatchesAll() {
		t.Error("empty selector should match all")
	}
	s = BranchSelector{MatchPatterns: []string{"main"}}
	if s.MatchesAll() {
		t.Error("non-empty selector should not match all")
	}
}

func TestResolveGitURL(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		mirror string
		want   string
	}{
		{
			name: "no mirror",
			url:  "https://github.com/puppetlabs/puppetlabs-stdlib.git",
			want: "https://github.com/puppetlabs/puppetlabs-stdlib.git",
		},
		{
			name:   "with mirror",
			url:    "https://github.com/puppetlabs/puppetlabs-stdlib.git",
			mirror: "https://mirror.example.com",
			want:   "https://mirror.example.com/puppetlabs/puppetlabs-stdlib.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Overrides: Overrides{GitMirror: tt.mirror}}
			got := cfg.ResolveGitURL(tt.url)
			if got != tt.want {
				t.Errorf("ResolveGitURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	base := &Config{
		CacheDir:       "/original/cache",
		EnvironmentDir: "/original/envs",
		ModuleSets: map[string][]Module{
			"base": {{Name: "stdlib", Git: "https://example.com/stdlib.git", Ref: "v1"}},
		},
		Sources: []Source{
			{URL: "https://example.com/repo1.git", BranchSelector: BranchSelector{}},
		},
	}

	other := &Config{
		CacheDir: "/new/cache",
		ModuleSets: map[string][]Module{
			"extra": {{Name: "apache", Git: "https://example.com/apache.git", Ref: "v2"}},
		},
		Sources: []Source{
			{URL: "https://example.com/repo2.git", BranchSelector: BranchSelector{}},
		},
		Environments: map[string]*Environment{
			"prod": {Ref: "main"},
		},
	}

	base.merge(other)

	if base.CacheDir != "/new/cache" {
		t.Errorf("CacheDir = %q, want /new/cache", base.CacheDir)
	}
	if base.EnvironmentDir != "/original/envs" {
		t.Errorf("EnvironmentDir = %q, want /original/envs", base.EnvironmentDir)
	}
	if len(base.ModuleSets) != 2 {
		t.Errorf("ModuleSets length = %d, want 2", len(base.ModuleSets))
	}
	if len(base.Sources) != 2 {
		t.Errorf("Sources length = %d, want 2", len(base.Sources))
	}
	if len(base.Environments) != 1 {
		t.Errorf("Environments length = %d, want 1", len(base.Environments))
	}
}
