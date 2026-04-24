package deployer

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/slauger/openvox-code/internal/cache"
	"github.com/slauger/openvox-code/internal/resolver"
)

// createTestRepo creates a local bare Git repository with a file and returns its file:// URL.
func createTestRepo(t *testing.T, dir, name, filename, content string) string {
	t.Helper()

	workDir := filepath.Join(dir, name+"-work")
	bareDir := filepath.Join(dir, name+".git")

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %s\n%s", args, err, out)
		}
	}

	run("git", "init", workDir)
	run("git", "-C", workDir, "config", "user.email", "test@test.com")
	run("git", "-C", workDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	run("git", "-C", workDir, "add", ".")
	run("git", "-C", workDir, "commit", "-m", "initial commit")
	run("git", "clone", "--bare", "--mirror", workDir, bareDir)

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
	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "init.pp", "class stdlib {}")
	apacheRepo := createTestRepo(t, repoDir, "apache", "init.pp", "class apache {}")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)

	// Clone repos into cache
	for _, repo := range []string{stdlibRepo, apacheRepo} {
		if err := cm.EnsureClone(repo); err != nil {
			t.Fatalf("EnsureClone(%s) error = %v", repo, err)
		}
	}

	// Resolve refs
	stdlibSHA, err := cm.ResolveRef(stdlibRepo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveRef(stdlib) error = %v", err)
	}
	apacheSHA, err := cm.ResolveRef(apacheRepo, "HEAD")
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

	d := New(envDir, cm, log)

	if err := d.DeployAll(envs, false); err != nil {
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
	content, err := os.ReadFile(filepath.Join(envDir, "production", "modules", "stdlib", "init.pp"))
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

	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "init.pp", "class stdlib {}")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)

	if err := cm.EnsureClone(stdlibRepo); err != nil {
		t.Fatal(err)
	}

	sha, err := cm.ResolveRef(stdlibRepo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// Create a stale environment
	if err := os.MkdirAll(filepath.Join(envDir, "stale-env", "modules"), 0o755); err != nil {
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

	d := New(envDir, cm, log)
	if err := d.DeployAll(envs, true); err != nil {
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

	stdlibRepo := createTestRepo(t, repoDir, "stdlib", "init.pp", "class stdlib {}")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)

	if err := cm.EnsureClone(stdlibRepo); err != nil {
		t.Fatal(err)
	}

	sha, err := cm.ResolveRef(stdlibRepo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// Create existing environment (should be replaced atomically)
	prodDir := filepath.Join(envDir, "production", "modules", "old-module")
	if err := os.MkdirAll(prodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(prodDir, "old.pp"), []byte("old"), 0o644)

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "production",
			Ref:  "main",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: stdlibRepo, SHA: sha},
			},
		},
	}

	d := New(envDir, cm, log)
	if err := d.DeployAll(envs, false); err != nil {
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
	entries, _ := os.ReadDir(envDir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover tmp directory: %s", e.Name())
		}
	}
}
