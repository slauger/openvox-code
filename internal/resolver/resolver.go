// Package resolver reads configuration and resolves Puppet environments with their modules.
package resolver

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/slauger/openvox-code/internal/config"
)

// BranchLister can list branches in a cached Git repository.
type BranchLister interface {
	ListBranches(ctx context.Context, gitURL string) ([]string, error)
}

// ResolvedModule represents a module ready for deployment.
type ResolvedModule struct {
	Name      string
	GitURL    string
	Ref       string
	SHA       string // Set after fetching, empty during pre-fetch resolve
	TargetDir string // Parent directory (default: "modules")
	InstallAs string // Directory name (default: Name)
}

// InstallPath returns the relative path where this module should be installed.
func (m *ResolvedModule) InstallPath() string {
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

// ResolvedEnvironment represents an environment with all modules resolved.
type ResolvedEnvironment struct {
	Name           string
	ControlRepoURL string
	Ref            string
	ModuleFiles    map[string]config.ModuleFileRequirement // path -> required|optional
	Modules        []ResolvedModule
}

// Resolver reads configuration and resolves environments.
type Resolver struct {
	cfg *config.Config
	log *slog.Logger
}

// New creates a new Resolver.
func New(cfg *config.Config, log *slog.Logger) *Resolver {
	return &Resolver{
		cfg: cfg,
		log: log,
	}
}

// Resolve resolves all environments from the configuration.
// For static environments, modules are resolved from module sets and inline definitions.
// Source-based (dynamic) environments are returned with their source URL for later branch discovery.
func (r *Resolver) Resolve() ([]ResolvedEnvironment, error) {
	var envs []ResolvedEnvironment

	// Resolve static environments
	for name, env := range r.cfg.Environments {
		resolved, err := r.resolveStaticEnvironment(name, env)
		if err != nil {
			return nil, fmt.Errorf("environment %q: %w", name, err)
		}
		envs = append(envs, resolved)
	}

	// Resolve source-based (dynamic) environments
	for _, src := range r.cfg.Sources {
		srcEnvs := r.resolveSourceEnvironments(src)
		envs = append(envs, srcEnvs...)
	}

	return envs, nil
}

// ExpandDiscovery replaces __source_discovery__ placeholders with actual branch-based
// environments by listing branches from the cache. This must be called after fetching.
func (r *Resolver) ExpandDiscovery(ctx context.Context, envs []ResolvedEnvironment, lister BranchLister) ([]ResolvedEnvironment, error) {
	var result []ResolvedEnvironment
	for _, env := range envs {
		if env.Name != "__source_discovery__" {
			result = append(result, env)
			continue
		}

		branches, err := lister.ListBranches(ctx, env.ControlRepoURL)
		if err != nil {
			return nil, fmt.Errorf("discovering branches for %s: %w", env.ControlRepoURL, err)
		}

		r.log.Info("discovered branches", "url", env.ControlRepoURL, "count", len(branches))
		for _, branch := range branches {
			r.log.Debug("discovered environment", "branch", branch, "url", env.ControlRepoURL)
			result = append(result, ResolvedEnvironment{
				Name:           branch,
				ControlRepoURL: env.ControlRepoURL,
				Ref:            branch,
				ModuleFiles:    env.ModuleFiles,
			})
		}
	}
	return result, nil
}

// Validate checks the configuration for correctness.
// When offline is true, only local validation is performed (no network checks).
func (r *Resolver) Validate(_ bool) error {
	// Schema validation is already done in config.Validate()
	// Here we check for logical consistency

	// Check for duplicate environment names
	seen := make(map[string]bool)
	for name := range r.cfg.Environments {
		if seen[name] {
			return fmt.Errorf("duplicate environment name %q", name)
		}
		seen[name] = true
	}

	// Check for duplicate modules within environments
	for name, env := range r.cfg.Environments {
		modules, err := r.collectModules(env)
		if err != nil {
			return fmt.Errorf("environment %q: %w", name, err)
		}
		modNames := make(map[string]bool)
		for _, m := range modules {
			if modNames[m.Name] {
				return fmt.Errorf("environment %q: duplicate module %q", name, m.Name)
			}
			modNames[m.Name] = true
		}
	}

	r.log.Debug("configuration validation passed")
	return nil
}

func (r *Resolver) resolveStaticEnvironment(name string, env *config.Environment) (ResolvedEnvironment, error) {
	modules, err := r.collectModules(env)
	if err != nil {
		return ResolvedEnvironment{}, err
	}

	return ResolvedEnvironment{
		Name:    name,
		Ref:     env.Ref,
		Modules: modules,
	}, nil
}

// buildModuleFiles merges the deprecated modulefile field with the new modulefiles map.
func buildModuleFiles(src config.Source) map[string]config.ModuleFileRequirement {
	mf := make(map[string]config.ModuleFileRequirement)

	// New format takes precedence
	for path, req := range src.ModuleFiles {
		mf[path] = req
	}

	// Backwards compatibility: old single modulefile field (treated as required)
	if src.ModuleFile != "" {
		if _, exists := mf[src.ModuleFile]; !exists {
			mf[src.ModuleFile] = config.ModuleFileRequired
		}
	}

	return mf
}

func (r *Resolver) resolveSourceEnvironments(src config.Source) []ResolvedEnvironment {
	gitURL := r.cfg.ResolveGitURL(src.URL)
	moduleFiles := buildModuleFiles(src)

	if src.Branches.All {
		// Branch discovery will happen after fetching
		return []ResolvedEnvironment{
			{
				Name:           "__source_discovery__",
				ControlRepoURL: gitURL,
				Ref:            "__all__",
				ModuleFiles:    moduleFiles,
			},
		}
	}

	envs := make([]ResolvedEnvironment, 0, len(src.Branches.Branches))
	for _, branch := range src.Branches.Branches {
		envs = append(envs, ResolvedEnvironment{
			Name:           branch,
			ControlRepoURL: gitURL,
			Ref:            branch,
			ModuleFiles:    moduleFiles,
		})
	}
	return envs
}

func (r *Resolver) collectModules(env *config.Environment) ([]ResolvedModule, error) {
	moduleMap := make(map[string]ResolvedModule)

	// Add modules from module sets
	for _, setName := range env.ModuleSets {
		modules, ok := r.cfg.ModuleSets[setName]
		if !ok {
			return nil, fmt.Errorf("unknown moduleset %q", setName)
		}
		for _, m := range modules {
			gitURL := r.cfg.ResolveGitURL(m.Git)
			moduleMap[m.Name] = ResolvedModule{
				Name:      m.Name,
				GitURL:    gitURL,
				Ref:       m.Ref,
				TargetDir: m.TargetDir,
				InstallAs: m.InstallAs,
			}
		}
	}

	// Add inline modules (override module set modules on conflict)
	for _, m := range env.Modules {
		gitURL := r.cfg.ResolveGitURL(m.Git)
		moduleMap[m.Name] = ResolvedModule{
			Name:      m.Name,
			GitURL:    gitURL,
			Ref:       m.Ref,
			TargetDir: m.TargetDir,
			InstallAs: m.InstallAs,
		}
	}

	modules := make([]ResolvedModule, 0, len(moduleMap))
	for _, m := range moduleMap {
		modules = append(modules, m)
	}
	return modules, nil
}
