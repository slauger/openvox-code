package resolver

import (
	"fmt"
	"log/slog"

	"github.com/slauger/openvox-code/internal/config"
)

// ResolvedModule represents a module ready for deployment.
type ResolvedModule struct {
	Name   string
	GitURL string
	Ref    string
	SHA    string // Set after fetching, empty during pre-fetch resolve
}

// ResolvedEnvironment represents an environment with all modules resolved.
type ResolvedEnvironment struct {
	Name           string
	ControlRepoURL string
	Ref            string
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
		srcEnvs, err := r.resolveSourceEnvironments(src)
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", src.URL, err)
		}
		envs = append(envs, srcEnvs...)
	}

	return envs, nil
}

// Validate checks the configuration for correctness.
func (r *Resolver) Validate(offline bool) error {
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

func (r *Resolver) resolveSourceEnvironments(src config.Source) ([]ResolvedEnvironment, error) {
	gitURL := r.cfg.ResolveGitURL(src.URL)

	if src.Branches.All {
		// Branch discovery will happen after fetching
		return []ResolvedEnvironment{
			{
				Name:           "__source_discovery__",
				ControlRepoURL: gitURL,
				Ref:            "__all__",
			},
		}, nil
	}

	var envs []ResolvedEnvironment
	for _, branch := range src.Branches.Branches {
		envs = append(envs, ResolvedEnvironment{
			Name:           branch,
			ControlRepoURL: gitURL,
			Ref:            branch,
		})
	}
	return envs, nil
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
				Name:   m.Name,
				GitURL: gitURL,
				Ref:    m.Ref,
			}
		}
	}

	// Add inline modules (override module set modules on conflict)
	for _, m := range env.Modules {
		gitURL := r.cfg.ResolveGitURL(m.Git)
		moduleMap[m.Name] = ResolvedModule{
			Name:   m.Name,
			GitURL: gitURL,
			Ref:    m.Ref,
		}
	}

	modules := make([]ResolvedModule, 0, len(moduleMap))
	for _, m := range moduleMap {
		modules = append(modules, m)
	}
	return modules, nil
}
