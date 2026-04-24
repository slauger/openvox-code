// Package config handles loading and validating openvox-code configuration files.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const (
	// APIVersion is the current config API version.
	APIVersion = "openvox.voxpupuli.org/v1alpha1"
	// KindCodeConfig is the kind for the main configuration.
	KindCodeConfig = "CodeConfig"
)

// Document represents a Kubernetes-style YAML document with apiVersion/kind envelope.
type Document struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Spec       Config    `yaml:"spec"`
	RawSpec    yaml.Node `yaml:"-"` // for lazy parsing
}

// Config represents the openvox-code configuration (the spec contents).
type Config struct {
	Includes       []string                `yaml:"includes,omitempty"`
	CacheDir       string                  `yaml:"cachedir"`
	EnvironmentDir string                  `yaml:"environmentdir"`
	Sources        []Source                `yaml:"sources,omitempty"`
	ModuleSets     map[string][]Module     `yaml:"modulesets,omitempty"`
	Environments   map[string]*Environment `yaml:"environments,omitempty"`
	Overrides      Overrides               `yaml:"overrides,omitempty"`
	Git            GitConfig               `yaml:"git,omitempty"`
	Offline        bool                    `yaml:"offline,omitempty"`
	OCI            *OCIConfig              `yaml:"oci,omitempty"`
}

// ModuleFileRef defines a module file to read from inside a control repo branch.
type ModuleFileRef struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
}

// Source defines a control repository for branch-based environment discovery.
type Source struct {
	URL            string          `yaml:"url"`
	BranchSelector BranchSelector  `yaml:"branchSelector"`
	ModuleFiles    []ModuleFileRef `yaml:"modulefiles,omitempty"`
}

// BranchSelector defines which branches to include or exclude using glob patterns.
type BranchSelector struct {
	MatchPatterns   []string `yaml:"matchPatterns,omitempty"`   // Glob patterns to include (empty = all)
	ExcludePatterns []string `yaml:"excludePatterns,omitempty"` // Glob patterns to exclude
}

// MatchesAll returns true if the selector matches all branches (no patterns specified).
func (bs *BranchSelector) MatchesAll() bool {
	return len(bs.MatchPatterns) == 0
}

// Matches returns true if the given branch name matches the selector.
func (bs *BranchSelector) Matches(branch string) bool {
	// If no match patterns, include everything
	if len(bs.MatchPatterns) == 0 {
		return !bs.isExcluded(branch)
	}

	// Check if branch matches any include pattern
	matched := false
	for _, pattern := range bs.MatchPatterns {
		if ok, _ := filepath.Match(pattern, branch); ok {
			matched = true
			break
		}
	}

	if !matched {
		return false
	}

	return !bs.isExcluded(branch)
}

func (bs *BranchSelector) isExcluded(branch string) bool {
	for _, pattern := range bs.ExcludePatterns {
		if ok, _ := filepath.Match(pattern, branch); ok {
			return true
		}
	}
	return false
}

// Module defines a Puppet module to be deployed.
type Module struct {
	Name         string `yaml:"name"`
	Git          string `yaml:"git"`
	Ref          string `yaml:"ref,omitempty"`           // Git ref (branch/tag/SHA). Empty = HEAD.
	FollowBranch bool   `yaml:"follow_branch,omitempty"` // Try environment branch first, then ref/HEAD as fallback
	TargetDir    string `yaml:"target_dir,omitempty"`    // Parent directory (default: "modules")
	InstallAs    string `yaml:"install_as,omitempty"`    // Directory name (default: Name)
}

// InstallPath returns the relative path where this module should be installed.
func (m *Module) InstallPath() string {
	dir := m.TargetDir
	if dir == "" {
		dir = "modules"
	}
	name := m.InstallAs
	if name == "" {
		name = m.Name
	}
	return dir + "/" + name
}

// Environment defines a static environment declaration.
type Environment struct {
	Ref        string   `yaml:"ref"`
	ModuleSets []string `yaml:"modulesets,omitempty"`
	Modules    []Module `yaml:"modules,omitempty"`
}

// KindModuleFile is the kind for per-branch module files.
const KindModuleFile = "ModuleFile"

// ModuleFileDocument is the K8s-style envelope for a module file.
type ModuleFileDocument struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Spec       ModuleFileSpec `yaml:"spec"`
}

// ModuleFileSpec is the contents of a per-branch module file.
type ModuleFileSpec struct {
	Modules []Module `yaml:"modules"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// ParseModuleFile parses a per-branch module file, supporting both K8s-style and flat formats.
func ParseModuleFile(data []byte) (*ModuleFileSpec, error) {
	var probe struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &probe); err == nil && probe.APIVersion != "" {
		if probe.APIVersion != APIVersion {
			return nil, fmt.Errorf("unsupported apiVersion %q (expected %q)", probe.APIVersion, APIVersion)
		}
		if probe.Kind != KindModuleFile {
			return nil, fmt.Errorf("unsupported kind %q (expected %q)", probe.Kind, KindModuleFile)
		}
		var doc ModuleFileDocument
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
		return &doc.Spec, nil
	}

	// Flat format fallback
	var spec ModuleFileSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// Overrides contains global override settings.
type Overrides struct {
	GitMirror  string            `yaml:"gitmirror,omitempty"`
	GitMirrors map[string]string `yaml:"gitmirrors,omitempty"`
}

// GitConfig holds Git authentication settings.
type GitConfig struct {
	SSHKeyPath       string `yaml:"ssh_key,omitempty"`
	SSHKnownHosts    string `yaml:"ssh_known_hosts,omitempty"`
	CredentialHelper string `yaml:"credential_helper,omitempty"`
}

// OCIConfig holds OCI image output configuration.
type OCIConfig struct {
	Registry   string `yaml:"registry"`
	Tag        string `yaml:"tag,omitempty"`
	AuthConfig string `yaml:"auth_config,omitempty"`
}

// Load reads and parses the configuration file.
// It supports both Kubernetes-style (apiVersion/kind/spec) and flat formats.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	if len(cfg.Includes) > 0 {
		baseDir := filepath.Dir(path)
		if err := cfg.processIncludes(baseDir); err != nil {
			return nil, fmt.Errorf("processing includes: %w", err)
		}
	}

	return cfg, nil
}

// parse detects format and parses configuration data.
func parse(data []byte) (*Config, error) {
	// Try Kubernetes-style first: peek for apiVersion field
	var probe struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &probe); err == nil && probe.APIVersion != "" {
		return parseK8sStyle(data, probe.APIVersion, probe.Kind)
	}

	// Fall back to flat format
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func parseK8sStyle(data []byte, apiVersion, kind string) (*Config, error) {
	if apiVersion != APIVersion {
		return nil, fmt.Errorf("unsupported apiVersion %q (expected %q)", apiVersion, APIVersion)
	}
	if kind != KindCodeConfig {
		return nil, fmt.Errorf("unsupported kind %q (expected %q)", kind, KindCodeConfig)
	}

	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc.Spec, nil
}

// processIncludes reads and merges included configuration files.
func (c *Config) processIncludes(baseDir string) error {
	for _, pattern := range c.Includes {
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(baseDir, pattern)
		}

		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("expanding glob %q: %w", pattern, err)
		}

		sort.Strings(matches)

		for _, match := range matches {
			data, err := os.ReadFile(filepath.Clean(match))
			if err != nil {
				return fmt.Errorf("reading include %q: %w", match, err)
			}

			inc, err := parse(data)
			if err != nil {
				return fmt.Errorf("parsing include %q: %w", match, err)
			}

			if len(inc.Includes) > 0 {
				return fmt.Errorf("include %q contains nested includes (not supported)", match)
			}

			c.merge(inc)
		}
	}
	return nil
}

// merge deep-merges another config into this one.
func (c *Config) merge(other *Config) {
	if other.CacheDir != "" {
		c.CacheDir = other.CacheDir
	}
	if other.EnvironmentDir != "" {
		c.EnvironmentDir = other.EnvironmentDir
	}
	if other.Overrides.GitMirror != "" {
		c.Overrides.GitMirror = other.Overrides.GitMirror
	}
	if len(other.Overrides.GitMirrors) > 0 {
		if c.Overrides.GitMirrors == nil {
			c.Overrides.GitMirrors = make(map[string]string)
		}
		for k, v := range other.Overrides.GitMirrors {
			c.Overrides.GitMirrors[k] = v
		}
	}
	if other.Git.SSHKeyPath != "" {
		c.Git.SSHKeyPath = other.Git.SSHKeyPath
	}
	if other.Git.SSHKnownHosts != "" {
		c.Git.SSHKnownHosts = other.Git.SSHKnownHosts
	}
	if other.Git.CredentialHelper != "" {
		c.Git.CredentialHelper = other.Git.CredentialHelper
	}
	if other.Offline {
		c.Offline = other.Offline
	}
	if other.OCI != nil {
		c.OCI = other.OCI
	}

	c.Sources = append(c.Sources, other.Sources...)

	if len(other.ModuleSets) > 0 {
		if c.ModuleSets == nil {
			c.ModuleSets = make(map[string][]Module)
		}
		for k, v := range other.ModuleSets {
			c.ModuleSets[k] = v
		}
	}

	if len(other.Environments) > 0 {
		if c.Environments == nil {
			c.Environments = make(map[string]*Environment)
		}
		for k, v := range other.Environments {
			c.Environments[k] = v
		}
	}
}

// Validate checks the configuration for required fields and consistency.
func (c *Config) Validate() error {
	if c.CacheDir == "" {
		return fmt.Errorf("cachedir is required")
	}
	if c.EnvironmentDir == "" {
		return fmt.Errorf("environmentdir is required")
	}

	if len(c.Sources) == 0 && len(c.Environments) == 0 {
		return fmt.Errorf("at least one source or environment must be defined")
	}

	for _, src := range c.Sources {
		if src.URL == "" {
			return fmt.Errorf("source url is required")
		}
	}

	for name, env := range c.Environments {
		if env.Ref == "" {
			return fmt.Errorf("environment %q: ref is required", name)
		}
		for _, msName := range env.ModuleSets {
			if _, ok := c.ModuleSets[msName]; !ok {
				return fmt.Errorf("environment %q: unknown moduleset %q", name, msName)
			}
		}
	}

	for setName, modules := range c.ModuleSets {
		seen := make(map[string]bool)
		for _, m := range modules {
			if m.Name == "" {
				return fmt.Errorf("moduleset %q: module name is required", setName)
			}
			if m.Git == "" {
				return fmt.Errorf("moduleset %q: module %q: git is required", setName, m.Name)
			}
			if seen[m.Name] {
				return fmt.Errorf("moduleset %q: duplicate module %q", setName, m.Name)
			}
			seen[m.Name] = true
		}
	}

	return nil
}

// ResolveGitURL applies mirror overrides to a Git URL if configured.
func (c *Config) ResolveGitURL(gitURL string) string {
	if len(c.Overrides.GitMirrors) > 0 {
		for host, mirror := range c.Overrides.GitMirrors {
			if hostMatches(gitURL, host) {
				return rewriteGitURL(gitURL, mirror)
			}
		}
	}

	if c.Overrides.GitMirror != "" {
		return rewriteGitURL(gitURL, c.Overrides.GitMirror)
	}
	return gitURL
}
