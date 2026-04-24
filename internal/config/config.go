package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level openvox-code configuration.
type Config struct {
	Includes       []string                `yaml:"includes,omitempty"`
	CacheDir       string                  `yaml:"cachedir"`
	EnvironmentDir string                  `yaml:"environmentdir"`
	Sources        []Source                `yaml:"sources,omitempty"`
	ModuleSets     map[string][]Module     `yaml:"modulesets,omitempty"`
	Environments   map[string]*Environment `yaml:"environments,omitempty"`
	Overrides      Overrides               `yaml:"overrides,omitempty"`
	Offline        bool                    `yaml:"offline,omitempty"`
	OCI            *OCIConfig              `yaml:"oci,omitempty"`
}

// Source defines a control repository for branch-based environment discovery.
type Source struct {
	URL        string      `yaml:"url"`
	Branches   BranchSpec  `yaml:"branches"`
	ModuleFile string      `yaml:"modulefile,omitempty"`
}

// BranchSpec can be "all" or a list of specific branch names.
type BranchSpec struct {
	All      bool
	Branches []string
}

func (b *BranchSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		if value.Value == "all" {
			b.All = true
			return nil
		}
		b.Branches = []string{value.Value}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var branches []string
		if err := value.Decode(&branches); err != nil {
			return fmt.Errorf("decoding branches: %w", err)
		}
		b.Branches = branches
		return nil
	}
	return fmt.Errorf("branches must be a string or list of strings")
}

// Module defines a Puppet module to be deployed.
type Module struct {
	Name string `yaml:"name"`
	Git  string `yaml:"git"`
	Ref  string `yaml:"ref"`
}

// Environment defines a static environment declaration.
type Environment struct {
	Ref        string   `yaml:"ref"`
	ModuleSets []string `yaml:"modulesets,omitempty"`
	Modules    []Module `yaml:"modules,omitempty"`
}

// Overrides contains global override settings.
type Overrides struct {
	GitMirror string `yaml:"gitmirror,omitempty"`
}

// OCIConfig holds OCI image output configuration.
type OCIConfig struct {
	Registry string `yaml:"registry"`
	Tag      string `yaml:"tag,omitempty"`
}

// Load reads and parses the configuration file, processing includes.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if len(cfg.Includes) > 0 {
		baseDir := filepath.Dir(path)
		if err := cfg.processIncludes(baseDir); err != nil {
			return nil, fmt.Errorf("processing includes: %w", err)
		}
	}

	return &cfg, nil
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
			data, err := os.ReadFile(match)
			if err != nil {
				return fmt.Errorf("reading include %q: %w", match, err)
			}

			var inc Config
			if err := yaml.Unmarshal(data, &inc); err != nil {
				return fmt.Errorf("parsing include %q: %w", match, err)
			}

			if len(inc.Includes) > 0 {
				return fmt.Errorf("include %q contains nested includes (not supported)", match)
			}

			c.merge(&inc)
		}
	}
	return nil
}

// merge deep-merges another config into this one.
func (c *Config) merge(other *Config) {
	// Scalars: later overrides earlier
	if other.CacheDir != "" {
		c.CacheDir = other.CacheDir
	}
	if other.EnvironmentDir != "" {
		c.EnvironmentDir = other.EnvironmentDir
	}
	if other.Overrides.GitMirror != "" {
		c.Overrides.GitMirror = other.Overrides.GitMirror
	}
	if other.Offline {
		c.Offline = other.Offline
	}
	if other.OCI != nil {
		c.OCI = other.OCI
	}

	// Lists: concatenate
	c.Sources = append(c.Sources, other.Sources...)

	// Maps: merge recursively
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
		if !src.Branches.All && len(src.Branches.Branches) == 0 {
			return fmt.Errorf("source %q: branches must be 'all' or a list", src.URL)
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
			if m.Ref == "" {
				return fmt.Errorf("moduleset %q: module %q: ref is required", setName, m.Name)
			}
			if seen[m.Name] {
				return fmt.Errorf("moduleset %q: duplicate module %q", setName, m.Name)
			}
			seen[m.Name] = true
		}
	}

	return nil
}

// ResolveGitURL applies the gitmirror override to a Git URL if configured.
func (c *Config) ResolveGitURL(url string) string {
	if c.Overrides.GitMirror == "" {
		return url
	}
	return rewriteGitURL(url, c.Overrides.GitMirror)
}
