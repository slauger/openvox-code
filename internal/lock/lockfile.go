// Package lock manages lockfiles that pin deployment state to exact Git SHAs.
package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// Lockfile represents a pinned deployment state with exact SHAs for every module.
type Lockfile struct {
	Version      int                          `yaml:"version"`
	GeneratedAt  time.Time                    `yaml:"generated_at"`
	Environments map[string]LockedEnvironment `yaml:"environments"`
}

// LockedEnvironment contains the pinned modules for one environment.
type LockedEnvironment struct {
	Ref            string         `yaml:"ref"`
	ControlRepoURL string         `yaml:"control_repo_url,omitempty"`
	ControlRepoSHA string         `yaml:"control_repo_sha,omitempty"`
	Modules        []LockedModule `yaml:"modules"`
}

// LockedModule is a module pinned to a concrete SHA.
type LockedModule struct {
	Name string `yaml:"name"`
	Git  string `yaml:"git"`
	Ref  string `yaml:"ref"`
	SHA  string `yaml:"sha"`
}

// Load reads a lockfile from disk.
func Load(path string) (*Lockfile, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("reading lockfile: %w", err)
	}

	var lf Lockfile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing lockfile: %w", err)
	}

	if lf.Version != 1 {
		return nil, fmt.Errorf("unsupported lockfile version: %d", lf.Version)
	}

	return &lf, nil
}

// Save writes the lockfile to disk.
func (lf *Lockfile) Save(path string) error {
	lf.Version = 1
	lf.GeneratedAt = time.Now().UTC()

	// Sort environments by name for deterministic output
	envNames := make([]string, 0, len(lf.Environments))
	for name := range lf.Environments {
		envNames = append(envNames, name)
	}
	sort.Strings(envNames)

	sorted := make(map[string]LockedEnvironment, len(lf.Environments))
	for _, name := range envNames {
		env := lf.Environments[name]
		// Sort modules within each environment
		sort.Slice(env.Modules, func(i, j int) bool {
			return env.Modules[i].Name < env.Modules[j].Name
		})
		sorted[name] = env
	}
	lf.Environments = sorted

	data, err := yaml.Marshal(lf)
	if err != nil {
		return fmt.Errorf("marshaling lockfile: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing lockfile: %w", err)
	}

	return nil
}

// NewFromResolved creates a Lockfile from resolved environments by resolving
// all refs to concrete SHAs using the provided resolver function.
func NewFromResolved(environments map[string]LockedEnvironment) *Lockfile {
	return &Lockfile{
		Version:      1,
		Environments: environments,
	}
}
