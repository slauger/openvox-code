package deployer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/slauger/openvox-code/internal/cache"
	"github.com/slauger/openvox-code/internal/config"
	"github.com/slauger/openvox-code/internal/resolver"
)

// runGit runs a git command for test setup.
func runGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- test-only
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, out)
	}
}

// createTestRepo creates a local bare Git repository with a file and returns its file:// URL.
func createTestRepo(t *testing.T, dir, name, content string) string {
	t.Helper()

	workDir := filepath.Join(dir, name+"-work")
	bareDir := filepath.Join(dir, name+".git")

	if err := os.MkdirAll(workDir, 0o750); err != nil {
		t.Fatal(err)
	}

	runGit(t, "init", "-b", "main", workDir)
	runGit(t, "-C", workDir, "config", "user.email", "test@test.com")
	runGit(t, "-C", workDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "init.pp"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	runGit(t, "-C", workDir, "add", ".")
	runGit(t, "-C", workDir, "commit", "-m", "initial commit")
	runGit(t, "clone", "--bare", "--mirror", workDir, bareDir)

	return bareDir
}

func TestDeployAllIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	// Create test module repos
	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "class stdlib {}")
	apacheRepo := createTestRepo(t, repoDir, "apache", "class apache {}")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	// Clone repos into cache
	for _, repo := range []string{stdlibRepo, apacheRepo} {
		if err := cm.EnsureClone(ctx, repo); err != nil {
			t.Fatalf("EnsureClone(%s) error = %v", repo, err)
		}
	}

	// Resolve refs
	stdlibSHA, err := cm.ResolveRef(ctx, stdlibRepo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveRef(stdlib) error = %v", err)
	}
	apacheSHA, err := cm.ResolveRef(ctx, apacheRepo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveRef(apache) error = %v", err)
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "production",
			Ref:  "main",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: stdlibRepo, Ref: "HEAD", SHA: stdlibSHA},
				{Name: "apache", GitURL: apacheRepo, Ref: "HEAD", SHA: apacheSHA},
			},
		},
	}

	d := New(envDir, cm, 4, log)

	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// Verify directory structure
	entries, err := os.ReadDir(envDir)
	if err != nil {
		t.Fatalf("reading envDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "production" {
		t.Fatalf("expected [production], got %v", entries)
	}

	// Verify modules
	modEntries, err := os.ReadDir(filepath.Join(envDir, "production", "modules"))
	if err != nil {
		t.Fatalf("reading modules dir: %v", err)
	}
	if len(modEntries) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modEntries))
	}

	// Verify file content
	content, err := os.ReadFile(filepath.Clean(filepath.Join(envDir, "production", "modules", "stdlib", "init.pp")))
	if err != nil {
		t.Fatalf("reading stdlib init.pp: %v", err)
	}
	if string(content) != "class stdlib {}" {
		t.Errorf("stdlib content = %q, want %q", string(content), "class stdlib {}")
	}
}

func TestDeployAllWithClean(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "class stdlib {}")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	if err := cm.EnsureClone(ctx, stdlibRepo); err != nil {
		t.Fatal(err)
	}

	sha, err := cm.ResolveRef(ctx, stdlibRepo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// Create a stale environment
	if err := os.MkdirAll(filepath.Join(envDir, "stale-env", "modules"), 0o750); err != nil {
		t.Fatal(err)
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "production",
			Ref:  "main",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: stdlibRepo, SHA: sha},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, true); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// Stale env should be removed
	if _, err := os.Stat(filepath.Join(envDir, "stale-env")); !os.IsNotExist(err) {
		t.Error("stale environment should have been removed")
	}

	// Production should exist
	if _, err := os.Stat(filepath.Join(envDir, "production")); err != nil {
		t.Error("production environment should exist")
	}
}

func TestDeployAtomicity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "class stdlib {}")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	if err := cm.EnsureClone(ctx, stdlibRepo); err != nil {
		t.Fatal(err)
	}

	sha, err := cm.ResolveRef(ctx, stdlibRepo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// Create existing environment (should be replaced atomically)
	prodDir := filepath.Join(envDir, "production", "modules", "old-module")
	if err := os.MkdirAll(prodDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prodDir, "old.pp"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "production",
			Ref:  "main",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: stdlibRepo, SHA: sha},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// Old module should be gone
	if _, err := os.Stat(filepath.Join(envDir, "production", "modules", "old-module")); !os.IsNotExist(err) {
		t.Error("old module should have been replaced")
	}

	// New module should exist
	if _, err := os.Stat(filepath.Join(envDir, "production", "modules", "stdlib", "init.pp")); err != nil {
		t.Error("new module should exist")
	}

	// No .tmp directories should remain
	entries, err := os.ReadDir(envDir)
	if err != nil {
		t.Fatalf("ReadDir error = %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover tmp directory: %s", e.Name())
		}
	}
}

// createTestControlRepo creates a bare repo with a control repo structure (manifests + optional module file).
func createTestControlRepo(t *testing.T, dir, moduleFileContent string) string {
	t.Helper()

	workDir := filepath.Join(dir, "control-work")
	bareDir := filepath.Join(dir, "control.git")

	if err := os.MkdirAll(filepath.Join(workDir, "manifests"), 0o750); err != nil {
		t.Fatal(err)
	}

	runGit(t, "init", "-b", "main", workDir)
	runGit(t, "-C", workDir, "config", "user.email", "test@test.com")
	runGit(t, "-C", workDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "manifests", "site.pp"), []byte("node default {}"), 0o600); err != nil {
		t.Fatal(err)
	}

	if moduleFileContent != "" {
		if err := os.WriteFile(filepath.Join(workDir, "modules.yaml"), []byte(moduleFileContent), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	runGit(t, "-C", workDir, "add", ".")
	runGit(t, "-C", workDir, "commit", "-m", "initial commit")
	runGit(t, "clone", "--bare", "--mirror", workDir, bareDir)

	return bareDir
}

func TestDeployControlRepoCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	controlRepo := createTestControlRepo(t, repoDir, "")
	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "class stdlib {}")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	for _, repo := range []string{controlRepo, stdlibRepo} {
		if err := cm.EnsureClone(ctx, repo); err != nil {
			t.Fatalf("EnsureClone(%s) error = %v", repo, err)
		}
	}

	stdlibSHA, err := cm.ResolveRef(ctx, stdlibRepo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name:           "production",
			ControlRepoURL: controlRepo,
			Ref:            "main",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: stdlibRepo, Ref: "HEAD", SHA: stdlibSHA},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// Verify control repo content (manifests/site.pp)
	content, err := os.ReadFile(filepath.Clean(filepath.Join(envDir, "production", "manifests", "site.pp")))
	if err != nil {
		t.Fatalf("reading site.pp: %v", err)
	}
	if string(content) != "node default {}" {
		t.Errorf("site.pp content = %q, want %q", string(content), "node default {}")
	}

	// Verify module deployed alongside control repo
	modContent, err := os.ReadFile(filepath.Clean(filepath.Join(envDir, "production", "modules", "stdlib", "init.pp")))
	if err != nil {
		t.Fatalf("reading stdlib init.pp: %v", err)
	}
	if string(modContent) != "class stdlib {}" {
		t.Errorf("stdlib content = %q", string(modContent))
	}
}

func TestDeployWithModuleFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	// Create a module repo
	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "class stdlib {}")

	// Create control repo with a modules.yaml
	moduleFileContent := "modules:\n  - name: stdlib\n    git: " + stdlibRepo + "\n    ref: main\n"
	controlRepo := createTestControlRepo(t, repoDir, moduleFileContent)

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	for _, repo := range []string{controlRepo, stdlibRepo} {
		if err := cm.EnsureClone(ctx, repo); err != nil {
			t.Fatalf("EnsureClone(%s) error = %v", repo, err)
		}
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name:           "production",
			ControlRepoURL: controlRepo,
			Ref:            "main",
			ModuleFiles: []config.ModuleFileRef{
				{Name: "modules.yaml", Required: true},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// Module from modules.yaml should be deployed
	if _, err := os.Stat(filepath.Join(envDir, "production", "modules", "stdlib", "init.pp")); err != nil {
		t.Errorf("stdlib module should exist: %v", err)
	}
}

func TestDeployWithModuleFileK8sStyle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "class stdlib {}")

	moduleFileContent := `apiVersion: openvox.voxpupuli.org/v1alpha1
kind: ModuleFile
spec:
  modules:
    - name: stdlib
      git: ` + stdlibRepo + `
      ref: main
`
	controlRepo := createTestControlRepo(t, repoDir, moduleFileContent)

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	for _, repo := range []string{controlRepo, stdlibRepo} {
		if err := cm.EnsureClone(ctx, repo); err != nil {
			t.Fatalf("EnsureClone(%s) error = %v", repo, err)
		}
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name:           "production",
			ControlRepoURL: controlRepo,
			Ref:            "main",
			ModuleFiles: []config.ModuleFileRef{
				{Name: "modules.yaml", Required: true},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(envDir, "production", "modules", "stdlib", "init.pp")); err != nil {
		t.Errorf("stdlib module should exist: %v", err)
	}
}

func TestDeployWithMissingRequiredModuleFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	// Control repo WITHOUT modules.yaml
	controlRepo := createTestControlRepo(t, repoDir, "")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	if err := cm.EnsureClone(ctx, controlRepo); err != nil {
		t.Fatal(err)
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name:           "production",
			ControlRepoURL: controlRepo,
			Ref:            "main",
			ModuleFiles: []config.ModuleFileRef{
				{Name: "modules.yaml", Required: true},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	err := d.DeployAll(ctx, envs, false)
	if err == nil {
		t.Fatal("expected error for missing required module file")
	}
}

func TestDeployWithOptionalMissingModuleFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	controlRepo := createTestControlRepo(t, repoDir, "")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	if err := cm.EnsureClone(ctx, controlRepo); err != nil {
		t.Fatal(err)
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name:           "production",
			ControlRepoURL: controlRepo,
			Ref:            "main",
			ModuleFiles: []config.ModuleFileRef{
				{Name: "modules.yaml", Required: false},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll should succeed with optional missing module file: %v", err)
	}
}

func TestDeployFollowBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	// Create a module repo with a "production" branch
	workDir := filepath.Join(repoDir, "stdlib-work")
	bareDir := filepath.Join(repoDir, "stdlib.git")

	if err := os.MkdirAll(workDir, 0o750); err != nil {
		t.Fatal(err)
	}

	runGit(t, "init", "-b", "main", workDir)
	runGit(t, "-C", workDir, "config", "user.email", "test@test.com")
	runGit(t, "-C", workDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "init.pp"), []byte("class stdlib { # main }"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, "-C", workDir, "add", ".")
	runGit(t, "-C", workDir, "commit", "-m", "initial on main")

	// Create production branch with different content
	runGit(t, "-C", workDir, "checkout", "-b", "production")
	if err := os.WriteFile(filepath.Join(workDir, "init.pp"), []byte("class stdlib { # production }"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, "-C", workDir, "add", ".")
	runGit(t, "-C", workDir, "commit", "-m", "production branch")

	runGit(t, "clone", "--bare", "--mirror", workDir, bareDir)

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	if err := cm.EnsureClone(ctx, bareDir); err != nil {
		t.Fatal(err)
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "production",
			Ref:  "main",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: bareDir, FollowBranch: true},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// Should have used the production branch (follow_branch)
	content, err := os.ReadFile(filepath.Clean(filepath.Join(envDir, "production", "modules", "stdlib", "init.pp")))
	if err != nil {
		t.Fatalf("reading init.pp: %v", err)
	}
	if string(content) != "class stdlib { # production }" {
		t.Errorf("content = %q, want production branch content", string(content))
	}
}

func TestDeployFollowBranchFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	// Create a module repo with only main branch (no "staging" branch)
	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "class stdlib { # main }")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	if err := cm.EnsureClone(ctx, stdlibRepo); err != nil {
		t.Fatal(err)
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "staging",
			Ref:  "staging",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: stdlibRepo, Ref: "main", FollowBranch: true},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// Should fall back to ref "main" since "staging" branch doesn't exist in module
	content, err := os.ReadFile(filepath.Clean(filepath.Join(envDir, "staging", "modules", "stdlib", "init.pp")))
	if err != nil {
		t.Fatalf("reading init.pp: %v", err)
	}
	if string(content) != "class stdlib { # main }" {
		t.Errorf("content = %q, want main branch content", string(content))
	}
}

func TestDeployParallelModules(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	// Create multiple module repos
	var repos []string
	var modules []resolver.ResolvedModule
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("mod%d", i)
		repo := createTestRepo(t, repoDir, name, fmt.Sprintf("class %s {}", name))
		repos = append(repos, repo)
		modules = append(modules, resolver.ResolvedModule{
			Name:   name,
			GitURL: repo,
			Ref:    "main",
		})
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	for _, repo := range repos {
		if err := cm.EnsureClone(ctx, repo); err != nil {
			t.Fatalf("EnsureClone error = %v", err)
		}
	}

	envs := []resolver.ResolvedEnvironment{
		{
			Name:    "production",
			Ref:     "main",
			Modules: modules,
		},
	}

	// Use parallel=2 to test the semaphore-based pool
	d := New(envDir, cm, 2, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// Verify all modules deployed
	modEntries, err := os.ReadDir(filepath.Join(envDir, "production", "modules"))
	if err != nil {
		t.Fatalf("reading modules dir: %v", err)
	}
	if len(modEntries) != 5 {
		t.Errorf("got %d modules, want 5", len(modEntries))
	}
}

func TestDeployWithModuleFileExclude(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	envDir := filepath.Join(tmpDir, "environments")

	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "class stdlib {}")
	apacheRepo := createTestRepo(t, repoDir, "apache", "class apache {}")

	// Module file that excludes stdlib
	moduleFileContent := "modules:\n  - name: apache\n    git: " + apacheRepo + "\n    ref: main\nexclude:\n  - stdlib\n"
	controlRepo := createTestControlRepo(t, repoDir, moduleFileContent)

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	ctx := context.Background()

	for _, repo := range []string{controlRepo, stdlibRepo, apacheRepo} {
		if err := cm.EnsureClone(ctx, repo); err != nil {
			t.Fatal(err)
		}
	}

	stdlibSHA, _ := cm.ResolveRef(ctx, stdlibRepo, "HEAD")

	envs := []resolver.ResolvedEnvironment{
		{
			Name:           "production",
			ControlRepoURL: controlRepo,
			Ref:            "main",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: stdlibRepo, Ref: "HEAD", SHA: stdlibSHA},
			},
			ModuleFiles: []config.ModuleFileRef{
				{Name: "modules.yaml", Required: true},
			},
		},
	}

	d := New(envDir, cm, 4, log)
	if err := d.DeployAll(ctx, envs, false); err != nil {
		t.Fatalf("DeployAll error = %v", err)
	}

	// stdlib should be excluded, apache should exist
	if _, err := os.Stat(filepath.Join(envDir, "production", "modules", "stdlib")); !os.IsNotExist(err) {
		t.Error("stdlib should have been excluded")
	}
	if _, err := os.Stat(filepath.Join(envDir, "production", "modules", "apache", "init.pp")); err != nil {
		t.Errorf("apache module should exist: %v", err)
	}
}
