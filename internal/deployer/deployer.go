// Package deployer handles atomic deployment of Puppet environments to disk.
package deployer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/slauger/openvox-code/internal/cache"
	"github.com/slauger/openvox-code/internal/resolver"
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
	envDir string
	cache  *cache.Manager
	log    *slog.Logger
}

// New creates a new Deployer.
func New(envDir string, cache *cache.Manager, log *slog.Logger) *Deployer {
	return &Deployer{
		envDir: envDir,
		cache:  cache,
		log:    log,
	}
}

// DeployAll deploys all resolved environments to disk using atomic operations.
func (d *Deployer) DeployAll(ctx context.Context, envs []resolver.ResolvedEnvironment, clean bool) error {
	if err := os.MkdirAll(d.envDir, 0o750); err != nil {
		return fmt.Errorf("creating environment directory: %w", err)
	}

	deployed := make(map[string]bool)

	for _, env := range envs {
		if env.Name == "__source_discovery__" {
			continue
		}

		d.log.Info("deploying environment", "name", env.Name)
		if err := d.deployEnvironment(ctx, env); err != nil {
			return fmt.Errorf("deploying %q: %w", env.Name, err)
		}
		deployed[env.Name] = true
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

func (d *Deployer) deployEnvironment(ctx context.Context, env resolver.ResolvedEnvironment) error {
	envPath := filepath.Join(d.envDir, env.Name)
	tmpPath := envPath + ".tmp"

	// Clean up any leftover temp directory
	if err := os.RemoveAll(tmpPath); err != nil {
		d.log.Warn("failed to remove leftover temp dir", "path", tmpPath, "error", err)
	}

	if err := os.MkdirAll(tmpPath, 0o750); err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}

	// Create modules directory
	modulesDir := filepath.Join(tmpPath, "modules")
	if err := os.MkdirAll(modulesDir, 0o750); err != nil {
		return fmt.Errorf("creating modules dir: %w", err)
	}

	// Deploy each module
	for _, mod := range env.Modules {
		modDir := filepath.Join(modulesDir, mod.Name)
		ref := mod.SHA
		if ref == "" {
			ref = mod.Ref
		}

		d.log.Debug("deploying module", "env", env.Name, "module", mod.Name, "ref", ref)
		if err := d.cache.Checkout(ctx, mod.GitURL, ref, modDir); err != nil {
			if rmErr := os.RemoveAll(tmpPath); rmErr != nil {
				d.log.Warn("failed to clean up temp dir after checkout error", "path", tmpPath, "error", rmErr)
			}
			return fmt.Errorf("checking out module %q: %w", mod.Name, err)
		}
	}

	// Atomic swap
	if err := os.RemoveAll(envPath); err != nil && !os.IsNotExist(err) {
		if rmErr := os.RemoveAll(tmpPath); rmErr != nil {
			d.log.Warn("failed to clean up temp dir after removal error", "path", tmpPath, "error", rmErr)
		}
		return fmt.Errorf("removing old environment: %w", err)
	}
	if err := os.Rename(tmpPath, envPath); err != nil {
		if rmErr := os.RemoveAll(tmpPath); rmErr != nil {
			d.log.Warn("failed to clean up temp dir after rename error", "path", tmpPath, "error", rmErr)
		}
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
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
