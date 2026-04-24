package cache

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// runGit runs a git command with fixed, known-safe subcommands for test setup.
func runGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- test-only
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, out)
	}
}

// createTestRepo creates a bare Git repo with a single commit and returns its path.
func createTestRepo(t *testing.T, dir string) string {
	t.Helper()

	workDir := filepath.Join(dir, "test-module-work")
	bareDir := filepath.Join(dir, "test-module.git")

	if err := os.MkdirAll(workDir, 0o750); err != nil {
		t.Fatal(err)
	}

	runGit(t, "init", "-b", "main", workDir)
	runGit(t, "-C", workDir, "config", "user.email", "test@test.com")
	runGit(t, "-C", workDir, "config", "user.name", "Test")

	// Create a file and commit
	if err := os.WriteFile(filepath.Join(workDir, "init.pp"), []byte("class test {}"), 0o600); err != nil {
		t.Fatal(err)
	}

	runGit(t, "-C", workDir, "add", ".")
	runGit(t, "-C", workDir, "commit", "-m", "initial commit")
	runGit(t, "clone", "--bare", workDir, bareDir)

	return bareDir
}

// createTestRepoWithBranches creates a bare Git repo with multiple branches.
func createTestRepoWithBranches(t *testing.T, dir, name string, branches []string) string {
	t.Helper()

	workDir := filepath.Join(dir, name+"-work")
	bareDir := filepath.Join(dir, name+".git")

	if err := os.MkdirAll(workDir, 0o750); err != nil {
		t.Fatal(err)
	}

	runGit(t, "init", "-b", "main", workDir)
	runGit(t, "-C", workDir, "config", "user.email", "test@test.com")
	runGit(t, "-C", workDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "init.pp"), []byte("class base {}"), 0o600); err != nil {
		t.Fatal(err)
	}

	runGit(t, "-C", workDir, "add", ".")
	runGit(t, "-C", workDir, "commit", "-m", "initial")

	for _, branch := range branches {
		runGit(t, "-C", workDir, "checkout", "-b", branch)

		if err := os.WriteFile(filepath.Join(workDir, branch+".pp"), []byte("class "+branch+" {}"), 0o600); err != nil {
			t.Fatal(err)
		}

		runGit(t, "-C", workDir, "add", ".")
		runGit(t, "-C", workDir, "commit", "-m", "add "+branch)
		runGit(t, "-C", workDir, "checkout", "main")
	}

	// Clone as bare
	runGit(t, "clone", "--bare", "--mirror", workDir, bareDir)

	return bareDir
}

func TestEnsureCloneAndFetch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")

	bareRepo := createTestRepo(t, repoDir)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)
	ctx := context.Background()

	// First clone
	if err := m.EnsureClone(ctx, bareRepo); err != nil {
		t.Fatalf("EnsureClone (initial) error = %v", err)
	}

	cachedPath := m.RepoPath(bareRepo)
	if _, err := os.Stat(filepath.Join(cachedPath, "HEAD")); err != nil {
		t.Fatalf("cached repo HEAD not found: %v", err)
	}

	// Second call should fetch (update)
	if err := m.EnsureClone(ctx, bareRepo); err != nil {
		t.Fatalf("EnsureClone (fetch) error = %v", err)
	}
}

func TestResolveRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")

	bareRepo := createTestRepo(t, repoDir)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)
	ctx := context.Background()

	if err := m.EnsureClone(ctx, bareRepo); err != nil {
		t.Fatalf("EnsureClone error = %v", err)
	}

	// Resolve HEAD
	sha, err := m.ResolveRef(ctx, bareRepo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveRef(HEAD) error = %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("SHA length = %d, want 40", len(sha))
	}
}

func TestResolveRefNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")

	bareRepo := createTestRepo(t, repoDir)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)
	ctx := context.Background()

	if err := m.EnsureClone(ctx, bareRepo); err != nil {
		t.Fatal(err)
	}

	_, err := m.ResolveRef(ctx, bareRepo, "nonexistent-branch")
	if err == nil {
		t.Fatal("expected error for nonexistent ref, got nil")
	}
}

func TestListBranches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")

	bareRepo := createTestRepoWithBranches(t, repoDir, "control-repo", []string{"staging", "development"})
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)
	ctx := context.Background()

	if err := m.EnsureClone(ctx, bareRepo); err != nil {
		t.Fatalf("EnsureClone error = %v", err)
	}

	branches, err := m.ListBranches(ctx, bareRepo)
	if err != nil {
		t.Fatalf("ListBranches error = %v", err)
	}

	// Should have at least main + staging + development
	if len(branches) < 3 {
		t.Errorf("got %d branches, want at least 3: %v", len(branches), branches)
	}

	branchSet := make(map[string]bool)
	for _, b := range branches {
		branchSet[b] = true
	}
	if !branchSet["staging"] {
		t.Error("staging branch not found")
	}
	if !branchSet["development"] {
		t.Error("development branch not found")
	}
}

func TestCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")
	targetDir := filepath.Join(tmpDir, "target")

	bareRepo := createTestRepo(t, repoDir)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)
	ctx := context.Background()

	if err := m.EnsureClone(ctx, bareRepo); err != nil {
		t.Fatalf("EnsureClone error = %v", err)
	}

	sha, err := m.ResolveRef(ctx, bareRepo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveRef error = %v", err)
	}

	if err := m.Checkout(ctx, bareRepo, sha, targetDir); err != nil {
		t.Fatalf("Checkout error = %v", err)
	}

	// Check that the file was checked out
	content, err := os.ReadFile(filepath.Clean(filepath.Join(targetDir, "init.pp")))
	if err != nil {
		t.Fatalf("reading checked out file: %v", err)
	}
	if string(content) != "class test {}" {
		t.Errorf("file content = %q, want %q", string(content), "class test {}")
	}
}

func TestEnsureCloneInvalidURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(tmpDir, log)

	err := m.EnsureClone(context.Background(), "https://invalid.example.com/nonexistent/repo.git")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}
