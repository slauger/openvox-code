package deployer

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/slauger/openvox-code/internal/cache"
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
