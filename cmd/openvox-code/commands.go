package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/slauger/openvox-code/internal/cache"
	"github.com/slauger/openvox-code/internal/config"
	"github.com/slauger/openvox-code/internal/deployer"
	"github.com/slauger/openvox-code/internal/fetcher"
	"github.com/slauger/openvox-code/internal/lock"
	"github.com/slauger/openvox-code/internal/resolver"
	"github.com/spf13/cobra"
)

func setupLogger() *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	if quiet {
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func resolveFromLockfile(path string) ([]resolver.ResolvedEnvironment, error) {
	lf, err := lock.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading lockfile: %w", err)
	}

	var envs []resolver.ResolvedEnvironment
	for name, env := range lf.Environments {
		re := resolver.ResolvedEnvironment{
			Name:           name,
			ControlRepoURL: env.ControlRepoURL,
			Ref:            env.Ref,
		}
		for _, mod := range env.Modules {
			re.Modules = append(re.Modules, resolver.ResolvedModule{
				Name:   mod.Name,
				GitURL: mod.Git,
				Ref:    mod.Ref,
				SHA:    mod.SHA,
			})
		}
		envs = append(envs, re)
	}
	return envs, nil
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("loading config %s: %w", cfgFile, err)
	}

	if cacheDir != "" {
		cfg.CacheDir = cacheDir
	}
	if environmentDir != "" {
		cfg.EnvironmentDir = environmentDir
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}
	return cfg, nil
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Fetch and deploy all environments",
	Long:  "Fetch all Git repositories and deploy environments to disk. Equivalent to running mirror followed by deploy.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := setupLogger()
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		cm := cache.New(cfg.CacheDir, log)
		d := deployer.New(cfg.EnvironmentDir, cm, log)

		if cfg.Offline {
			return fmt.Errorf("sync requires network access; use 'deploy' in offline mode")
		}

		f := fetcher.New(cm, parallel, log)
		r := resolver.New(cfg, log)

		log.Info("starting mirror phase")
		resolved, err := r.Resolve()
		if err != nil {
			return fmt.Errorf("resolving environments: %w", err)
		}

		if err := f.FetchAll(resolved); err != nil {
			return fmt.Errorf("fetching repositories: %w", err)
		}

		log.Info("starting deploy phase")
		clean, _ := cmd.Flags().GetBool("clean")
		if err := d.DeployAll(resolved, clean); err != nil {
			return fmt.Errorf("deploying environments: %w", err)
		}

		log.Info("sync complete")
		return nil
	},
}

var mirrorCmd = &cobra.Command{
	Use:   "mirror",
	Short: "Fetch and cache all Git repositories",
	Long:  "Fetch all Git repositories into the local bare clone cache without deploying.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := setupLogger()
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if cfg.Offline {
			return fmt.Errorf("mirror requires network access; cannot run in offline mode")
		}

		cm := cache.New(cfg.CacheDir, log)
		f := fetcher.New(cm, parallel, log)
		r := resolver.New(cfg, log)

		resolved, err := r.Resolve()
		if err != nil {
			return fmt.Errorf("resolving environments: %w", err)
		}

		if err := f.FetchAll(resolved); err != nil {
			return fmt.Errorf("fetching repositories: %w", err)
		}

		log.Info("mirror complete")
		return nil
	},
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy environments from cache",
	Long:  "Deploy environments from the local bare clone cache. No network access required.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := setupLogger()
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		cm := cache.New(cfg.CacheDir, log)
		d := deployer.New(cfg.EnvironmentDir, cm, log)

		var resolved []resolver.ResolvedEnvironment

		if lockfilePath != "" {
			log.Info("deploying from lockfile", "path", lockfilePath)
			resolved, err = resolveFromLockfile(lockfilePath)
			if err != nil {
				return err
			}
		} else {
			r := resolver.New(cfg, log)
			resolved, err = r.Resolve()
			if err != nil {
				return fmt.Errorf("resolving environments: %w", err)
			}
		}

		clean, _ := cmd.Flags().GetBool("clean")
		if err := d.DeployAll(resolved, clean); err != nil {
			return fmt.Errorf("deploying environments: %w", err)
		}

		log.Info("deploy complete")
		return nil
	},
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show what would change",
	Long:  "Compare the current deployed state with what a sync would produce.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := setupLogger()
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		r := resolver.New(cfg, log)
		d := deployer.New(cfg.EnvironmentDir, nil, log)

		resolved, err := r.Resolve()
		if err != nil {
			return fmt.Errorf("resolving environments: %w", err)
		}

		changes, err := d.Diff(resolved)
		if err != nil {
			return fmt.Errorf("computing diff: %w", err)
		}

		if len(changes) == 0 {
			fmt.Println("No changes detected.")
			return nil
		}

		for _, c := range changes {
			fmt.Println(c)
		}
		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  "Validate the configuration file syntax and check that all referenced Git refs are reachable.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := setupLogger()
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		r := resolver.New(cfg, log)
		offline, _ := cmd.Flags().GetBool("offline")

		if err := r.Validate(offline); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		log.Info("configuration is valid")
		return nil
	},
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build OCI container image",
	Long:  "Build an OCI container image containing the deployed environments.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("build command not yet implemented (planned for v0.3)")
	},
}

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Generate or update lockfile",
	Long:  "Resolve all refs to concrete Git SHAs and write a lockfile.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := setupLogger()
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		cm := cache.New(cfg.CacheDir, log)
		f := fetcher.New(cm, parallel, log)
		r := resolver.New(cfg, log)

		resolved, err := r.Resolve()
		if err != nil {
			return fmt.Errorf("resolving environments: %w", err)
		}

		// Fetch all repos to resolve refs to SHAs
		if err := f.FetchAll(resolved); err != nil {
			return fmt.Errorf("fetching repositories: %w", err)
		}

		// Build lockfile
		lockedEnvs := make(map[string]lock.LockedEnvironment)
		for _, env := range resolved {
			if env.Name == "__source_discovery__" {
				continue
			}

			locked := lock.LockedEnvironment{
				Ref:            env.Ref,
				ControlRepoURL: env.ControlRepoURL,
			}

			if env.ControlRepoURL != "" {
				sha, err := cm.ResolveRef(env.ControlRepoURL, env.Ref)
				if err != nil {
					return fmt.Errorf("resolving control repo ref for %q: %w", env.Name, err)
				}
				locked.ControlRepoSHA = sha
			}

			for _, mod := range env.Modules {
				sha, err := cm.ResolveRef(mod.GitURL, mod.Ref)
				if err != nil {
					return fmt.Errorf("resolving ref for %s/%s: %w", env.Name, mod.Name, err)
				}
				locked.Modules = append(locked.Modules, lock.LockedModule{
					Name: mod.Name,
					Git:  mod.GitURL,
					Ref:  mod.Ref,
					SHA:  sha,
				})
			}
			lockedEnvs[env.Name] = locked
		}

		lf := lock.NewFromResolved(lockedEnvs)

		outPath := lockfilePath
		if outPath == "" {
			outPath = "openvox-code.lock"
		}

		if err := lf.Save(outPath); err != nil {
			return fmt.Errorf("saving lockfile: %w", err)
		}

		log.Info("lockfile written", "path", outPath)
		return nil
	},
}

func init() {
	syncCmd.Flags().Bool("clean", false, "remove environments not in config")
	deployCmd.Flags().Bool("clean", false, "remove environments not in config")
	validateCmd.Flags().Bool("offline", false, "skip network checks, validate syntax only")
	buildCmd.Flags().String("tag", "", "image tag")
	buildCmd.Flags().String("registry", "", "override OCI registry URL")
	buildCmd.Flags().Bool("push", false, "push image after building")
}
