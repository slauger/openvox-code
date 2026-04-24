// Package deployer handles atomic deployment of Puppet environments to disk.
package deployer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/slauger/openvox-code/internal/cache"
	"github.com/slauger/openvox-code/internal/config"
	"github.com/slauger/openvox-code/internal/resolver"
	"gopkg.in/yaml.v3"
)

// Change describes a deployment difference.
type Change struct {
	Type        string // "add", "remove", "update"
	Environment string
	Module      string
	OldRef      string
	NewRef      string
}

func (c Change) String() string {
	switch c.Type {
	case "add":
		if c.Module != "" {
			return fmt.Sprintf("+ %s/%s (%s)", c.Environment, c.Module, c.NewRef)
		}
		return fmt.Sprintf("+ %s", c.Environment)
	case "remove":
		if c.Module != "" {
			return fmt.Sprintf("- %s/%s", c.Environment, c.Module)
		}
		return fmt.Sprintf("- %s", c.Environment)
	case "update":
		return fmt.Sprintf("~ %s/%s (%s -> %s)", c.Environment, c.Module, c.OldRef, c.NewRef)
	default:
		return fmt.Sprintf("? %s/%s", c.Environment, c.Module)
	}
}

// Deployer handles atomic deployment of environments to disk.
type Deployer struct {
	envDir   string
	cache    *cache.Manager
	parallel int
	log      *slog.Logger
}

// New creates a new Deployer.
func New(envDir string, cm *cache.Manager, parallel int, log *slog.Logger) *Deployer {
	if parallel < 1 {
		parallel = runtime.NumCPU()
	}
	return &Deployer{
		envDir:   envDir,
		cache:    cm,
		parallel: parallel,
		log:      log,
	}
}

// DeployAll deploys all resolved environments to disk using atomic operations.
func (d *Deployer) DeployAll(ctx context.Context, envs []resolver.ResolvedEnvironment, clean bool) error {
	if err := os.MkdirAll(d.envDir, 0o750); err != nil {
		return fmt.Errorf("creating environment directory: %w", err)
	}

	deployed := make(map[string]bool)

	for i := range envs {
		if envs[i].Name == "__source_discovery__" {
			continue
		}

		d.log.Info("deploying environment", "name", envs[i].Name)
		if err := d.deployEnvironment(ctx, &envs[i]); err != nil {
			return fmt.Errorf("deploying %q: %w", envs[i].Name, err)
		}
		deployed[envs[i].Name] = true
	}

	if clean {
		if err := d.cleanStale(deployed); err != nil {
			return fmt.Errorf("cleaning stale environments: %w", err)
		}
	}

	return nil
}

// Diff computes the changes that would result from deploying the given environments.
func (d *Deployer) Diff(envs []resolver.ResolvedEnvironment) ([]Change, error) {
	var changes []Change

	existing, err := d.listExistingEnvironments()
	if err != nil {
		return nil, err
	}

	expected := make(map[string]bool)
	for _, env := range envs {
		if env.Name == "__source_discovery__" {
			continue
		}
		expected[env.Name] = true

		envPath := filepath.Join(d.envDir, env.Name)
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			changes = append(changes, Change{Type: "add", Environment: env.Name})
			for _, mod := range env.Modules {
				changes = append(changes, Change{
					Type:        "add",
					Environment: env.Name,
					Module:      mod.Name,
					NewRef:      mod.Ref,
				})
			}
			continue
		}

		// Check modules
		existingMods, err := d.listExistingModules(env.Name)
		if err != nil {
			return nil, err
		}

		expectedMods := make(map[string]bool)
		for _, mod := range env.Modules {
			expectedMods[mod.Name] = true
			if !existingMods[mod.Name] {
				changes = append(changes, Change{
					Type:        "add",
					Environment: env.Name,
					Module:      mod.Name,
					NewRef:      mod.Ref,
				})
			}
		}

		for modName := range existingMods {
			if !expectedMods[modName] {
				changes = append(changes, Change{
					Type:        "remove",
					Environment: env.Name,
					Module:      modName,
				})
			}
		}
	}

	for _, name := range existing {
		if !expected[name] {
			changes = append(changes, Change{Type: "remove", Environment: name})
		}
	}

	return changes, nil
}

func (d *Deployer) deployEnvironment(ctx context.Context, env *resolver.ResolvedEnvironment) error {
	envPath := filepath.Join(d.envDir, env.Name)
	tmpPath := envPath + ".tmp"

	cleanup := func() {
		if err := os.RemoveAll(tmpPath); err != nil {
			d.log.Warn("failed to clean up temp dir", "path", tmpPath, "error", err)
		}
	}

	// Clean up any leftover temp directory
	cleanup()

	if err := os.MkdirAll(tmpPath, 0o750); err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}

	// Step 1: If this environment comes from a control repo source, checkout the
	// control repo as the base of the environment (manifests/, hieradata/, etc.)
	if env.ControlRepoURL != "" {
		ref := env.Ref
		d.log.Debug("checking out control repo", "env", env.Name, "url", env.ControlRepoURL, "ref", ref)
		if err := d.cache.Checkout(ctx, env.ControlRepoURL, ref, tmpPath); err != nil {
			cleanup()
			return fmt.Errorf("checking out control repo for %q: %w", env.Name, err)
		}
	}

	// Step 2: Read per-environment module files from the checked-out control repo
	// These modules are merged with (and override) the globally-resolved modules.
	modules := env.Modules
	if len(env.ModuleFiles) > 0 && env.ControlRepoURL != "" {
		for _, mf := range env.ModuleFiles {
			parsed, err := d.readModuleFile(filepath.Join(tmpPath, filepath.Clean(mf.Name)))
			if err != nil {
				if mf.Required {
					cleanup()
					return fmt.Errorf("required module file %q not found in %s/%s: %w", mf.Name, env.Name, env.Ref, err)
				}
				d.log.Debug("optional module file not found", "env", env.Name, "path", mf.Name)
				continue
			}
			modules = mergeModules(modules, parsed.modules, parsed.exclude)
			d.log.Info("loaded module file", "env", env.Name, "path", mf.Name,
				"modules", len(parsed.modules), "excluded", len(parsed.exclude))
		}
	}

	// Step 3: Create all target directories first (must be sequential for mkdir)
	for i := range modules {
		modDir := filepath.Join(tmpPath, modules[i].InstallPath())
		if err := os.MkdirAll(filepath.Dir(modDir), 0o750); err != nil {
			cleanup()
			return fmt.Errorf("creating parent dir for module %q: %w", modules[i].Name, err)
		}
	}

	// Step 4: Deploy modules in parallel
	var (
		wg      sync.WaitGroup
		errChan = make(chan error, len(modules))
		sem     = make(chan struct{}, d.parallel)
	)

	for i := range modules {
		mod := &modules[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Ensure the module repo is cached (may not be if discovered via modules.yaml)
			if err := d.cache.EnsureClone(ctx, mod.GitURL); err != nil {
				errChan <- fmt.Errorf("fetching module %q from %s: %w", mod.Name, mod.GitURL, err)
				return
			}

			ref, err := d.resolveModuleRef(ctx, mod, env.Name)
			if err != nil {
				errChan <- fmt.Errorf("resolving ref for module %q: %w", mod.Name, err)
				return
			}

			modDir := filepath.Join(tmpPath, mod.InstallPath())
			d.log.Debug("deploying module", "env", env.Name, "module", mod.Name, "ref", ref, "path", mod.InstallPath())
			if err := d.cache.Checkout(ctx, mod.GitURL, ref, modDir); err != nil {
				errChan <- fmt.Errorf("checking out module %q: %w", mod.Name, err)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		cleanup()
		return err
	}

	// Atomic swap
	if err := os.RemoveAll(envPath); err != nil && !os.IsNotExist(err) {
		cleanup()
		return fmt.Errorf("removing old environment: %w", err)
	}
	if err := os.Rename(tmpPath, envPath); err != nil {
		cleanup()
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
}

// resolveModuleRef determines the Git ref to check out for a module.
//
// Priority:
//  1. If SHA is set (from lockfile), use it directly
//  2. If follow_branch is true, try the environment name as branch
//  3. Use the explicit ref field
//  4. If ref is empty, use HEAD
func (d *Deployer) resolveModuleRef(ctx context.Context, mod *resolver.ResolvedModule, envName string) (string, error) {
	// Lockfile-pinned SHA always wins
	if mod.SHA != "" {
		return mod.SHA, nil
	}

	// follow_branch: try environment name as branch first
	if mod.FollowBranch {
		_, err := d.cache.ResolveRef(ctx, mod.GitURL, envName)
		if err == nil {
			d.log.Debug("follow_branch: using environment branch", "module", mod.Name, "branch", envName)
			return envName, nil
		}
		// Branch doesn't exist in module repo — fall through to ref/HEAD
		d.log.Debug("follow_branch: environment branch not found, using fallback", "module", mod.Name, "branch", envName)
	}

	// Explicit ref
	if mod.Ref != "" {
		if _, err := d.cache.ResolveRef(ctx, mod.GitURL, mod.Ref); err != nil {
			return "", fmt.Errorf("ref %q not found in %s: %w", mod.Ref, mod.GitURL, err)
		}
		return mod.Ref, nil
	}

	// Default: HEAD
	if _, err := d.cache.ResolveRef(ctx, mod.GitURL, "HEAD"); err != nil {
		return "", fmt.Errorf("HEAD not found in %s (empty repo?): %w", mod.GitURL, err)
	}
	return "HEAD", nil
}

// parsedModuleFile holds the parsed result of a per-branch module file.
type parsedModuleFile struct {
	modules []resolver.ResolvedModule
	exclude map[string]bool
}

// readModuleFile reads a YAML module list from a file inside a checked-out environment.
func (d *Deployer) readModuleFile(path string) (*parsedModuleFile, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	var mf config.ModuleFile
	if err := yaml.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("parsing module file: %w", err)
	}

	modules := make([]resolver.ResolvedModule, 0, len(mf.Modules))
	for _, m := range mf.Modules {
		modules = append(modules, resolver.ResolvedModule{
			Name:         m.Name,
			GitURL:       m.Git,
			Ref:          m.Ref,
			FollowBranch: m.FollowBranch,
			TargetDir:    m.TargetDir,
			InstallAs:    m.InstallAs,
		})
	}

	exclude := make(map[string]bool, len(mf.Exclude))
	for _, name := range mf.Exclude {
		exclude[name] = true
	}

	return &parsedModuleFile{modules: modules, exclude: exclude}, nil
}

// mergeModules merges environment-local modules into global modules.
// Local modules override global modules with the same name.
// Excluded module names are removed from the result.
func mergeModules(global, local []resolver.ResolvedModule, exclude map[string]bool) []resolver.ResolvedModule {
	merged := make(map[string]resolver.ResolvedModule)
	for _, m := range global {
		if !exclude[m.Name] {
			merged[m.Name] = m
		}
	}
	for _, m := range local {
		merged[m.Name] = m
	}
	result := make([]resolver.ResolvedModule, 0, len(merged))
	for _, m := range merged {
		result = append(result, m)
	}
	return result
}

func (d *Deployer) cleanStale(deployed map[string]bool) error {
	existing, err := d.listExistingEnvironments()
	if err != nil {
		return err
	}

	for _, name := range existing {
		if !deployed[name] {
			envPath := filepath.Join(d.envDir, name)
			d.log.Info("removing stale environment", "name", name)
			if err := os.RemoveAll(envPath); err != nil {
				return fmt.Errorf("removing stale environment %q: %w", name, err)
			}
		}
	}
	return nil
}

func (d *Deployer) listExistingEnvironments() ([]string, error) {
	entries, err := os.ReadDir(d.envDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading environment directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && !strings.HasSuffix(entry.Name(), ".tmp") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (d *Deployer) listExistingModules(envName string) (map[string]bool, error) {
	modulesDir := filepath.Join(d.envDir, envName, "modules")
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading modules dir: %w", err)
	}

	mods := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			mods[entry.Name()] = true
		}
	}
	return mods, nil
}
