package cache

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// createTestRepo creates a bare Git repo with a single commit and returns its path.
func createTestRepo(t *testing.T, dir, name string) string {
	t.Helper()

	workDir := filepath.Join(dir, name+"-work")
	bareDir := filepath.Join(dir, name+".git")

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init", workDir},
		{"git", "-C", workDir, "config", "user.email", "test@test.com"},
		{"git", "-C", workDir, "config", "user.name", "Test"},
	}
	for _, c := range cmds {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %s\n%s", c, err, out)
		}
	}

	// Create a file and commit
	if err := os.WriteFile(filepath.Join(workDir, "init.pp"), []byte("class test {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds = [][]string{
		{"git", "-C", workDir, "add", "."},
		{"git", "-C", workDir, "commit", "-m", "initial commit"},
		{"git", "clone", "--bare", workDir, bareDir},
	}
	for _, c := range cmds {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %s\n%s", c, err, out)
		}
	}

	return bareDir
}

// createTestRepoWithBranches creates a bare Git repo with multiple branches.
func createTestRepoWithBranches(t *testing.T, dir, name string, branches []string) string {
	t.Helper()

	workDir := filepath.Join(dir, name+"-work")
	bareDir := filepath.Join(dir, name+".git")

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init", workDir},
		{"git", "-C", workDir, "config", "user.email", "test@test.com"},
		{"git", "-C", workDir, "config", "user.name", "Test"},
	}
	for _, c := range cmds {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %s\n%s", c, err, out)
		}
	}

	if err := os.WriteFile(filepath.Join(workDir, "init.pp"), []byte("class base {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds = [][]string{
		{"git", "-C", workDir, "add", "."},
		{"git", "-C", workDir, "commit", "-m", "initial"},
	}
	for _, c := range cmds {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %s\n%s", c, err, out)
		}
	}

	for _, branch := range branches {
		cmds := [][]string{
			{"git", "-C", workDir, "checkout", "-b", branch},
		}

		if err := os.WriteFile(filepath.Join(workDir, branch+".pp"), []byte("class "+branch+" {}"), 0o644); err != nil {
			t.Fatal(err)
		}

		cmds = append(cmds,
			[]string{"git", "-C", workDir, "add", "."},
			[]string{"git", "-C", workDir, "commit", "-m", "add " + branch},
			[]string{"git", "-C", workDir, "checkout", "main"},
		)

		for _, c := range cmds {
			out, err := exec.Command(c[0], c[1:]...).CombinedOutput()
			if err != nil {
				// Try master instead of main for older git
				if strings.Contains(string(out), "not a valid") && c[len(c)-1] == "main" {
					c[len(c)-1] = "master"
					if out2, err2 := exec.Command(c[0], c[1:]...).CombinedOutput(); err2 != nil {
						t.Fatalf("command %v failed: %s\n%s", c, err2, out2)
					}
					continue
				}
				t.Fatalf("command %v failed: %s\n%s", c, err, out)
			}
		}
	}

	// Clone as bare
	if out, err := exec.Command("git", "clone", "--bare", "--mirror", workDir, bareDir).CombinedOutput(); err != nil {
		t.Fatalf("bare clone failed: %s\n%s", err, out)
	}

	return bareDir
}

func TestEnsureCloneAndFetch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")

	bareRepo := createTestRepo(t, repoDir, "test-module")
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)

	// First clone
	if err := m.EnsureClone(bareRepo); err != nil {
		t.Fatalf("EnsureClone (initial) error = %v", err)
	}

	cachedPath := m.RepoPath(bareRepo)
	if _, err := os.Stat(filepath.Join(cachedPath, "HEAD")); err != nil {
		t.Fatalf("cached repo HEAD not found: %v", err)
	}

	// Second call should fetch (update)
	if err := m.EnsureClone(bareRepo); err != nil {
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

	bareRepo := createTestRepo(t, repoDir, "test-module")
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)

	if err := m.EnsureClone(bareRepo); err != nil {
		t.Fatalf("EnsureClone error = %v", err)
	}

	// Resolve HEAD
	sha, err := m.ResolveRef(bareRepo, "HEAD")
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

	bareRepo := createTestRepo(t, repoDir, "test-module")
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)

	if err := m.EnsureClone(bareRepo); err != nil {
		t.Fatal(err)
	}

	_, err := m.ResolveRef(bareRepo, "nonexistent-branch")
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

	if err := m.EnsureClone(bareRepo); err != nil {
		t.Fatalf("EnsureClone error = %v", err)
	}

	branches, err := m.ListBranches(bareRepo)
	if err != nil {
		t.Fatalf("ListBranches error = %v", err)
	}

	// Should have at least main/master + staging + development
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

	bareRepo := createTestRepo(t, repoDir, "test-module")
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := New(cacheDir, log)

	if err := m.EnsureClone(bareRepo); err != nil {
		t.Fatalf("EnsureClone error = %v", err)
	}

	sha, err := m.ResolveRef(bareRepo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveRef error = %v", err)
	}

	if err := m.Checkout(bareRepo, sha, targetDir); err != nil {
		t.Fatalf("Checkout error = %v", err)
	}

	// Check that the file was checked out
	content, err := os.ReadFile(filepath.Join(targetDir, "init.pp"))
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

	err := m.EnsureClone("https://invalid.example.com/nonexistent/repo.git")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}
