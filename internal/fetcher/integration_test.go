package fetcher

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

func createTestRepo(t *testing.T, dir, name string) string {
	t.Helper()

	workDir := filepath.Join(dir, name+"-work")
	bareDir := filepath.Join(dir, name+".git")

	if err := os.MkdirAll(workDir, 0o750); err != nil {
		t.Fatal(err)
	}

	runGit(t, "init", "-b", "main", workDir)
	runGit(t, "-C", workDir, "config", "user.email", "test@test.com")
	runGit(t, "-C", workDir, "config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(workDir, "init.pp"), []byte("class "+name+" {}"), 0o600); err != nil {
		t.Fatal(err)
	}

	runGit(t, "-C", workDir, "add", ".")
	runGit(t, "-C", workDir, "commit", "-m", "initial")
	runGit(t, "clone", "--bare", "--mirror", workDir, bareDir)

	return bareDir
}

func TestFetchAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")

	repo1 := createTestRepo(t, repoDir, "stdlib")
	repo2 := createTestRepo(t, repoDir, "apache")
	repo3 := createTestRepo(t, repoDir, "concat")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	f := New(cm, 4, log)
	ctx := context.Background()

	envs := []resolver.ResolvedEnvironment{
		{
			Name: "production",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: repo1},
				{Name: "apache", GitURL: repo2},
			},
		},
		{
			Name: "staging",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: repo1},
				{Name: "concat", GitURL: repo3},
			},
		},
	}

	if err := f.FetchAll(ctx, envs); err != nil {
		t.Fatalf("FetchAll error = %v", err)
	}

	// Verify all repos are cached
	for _, url := range []string{repo1, repo2, repo3} {
		path := cm.RepoPath(url)
		if _, err := os.Stat(filepath.Join(path, "HEAD")); err != nil {
			t.Errorf("repo %s not cached at %s", url, path)
		}
	}
}

func TestFetchAllEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(tmpDir, log)
	f := New(cm, 2, log)

	if err := f.FetchAll(context.Background(), nil); err != nil {
		t.Fatalf("FetchAll(nil) error = %v", err)
	}
}

func TestFetchAllWithControlRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")

	controlRepo := createTestRepo(t, repoDir, "control")
	moduleRepo := createTestRepo(t, repoDir, "module")

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	f := New(cm, 2, log)
	ctx := context.Background()

	envs := []resolver.ResolvedEnvironment{
		{
			Name:           "production",
			ControlRepoURL: controlRepo,
			Modules: []resolver.ResolvedModule{
				{Name: "module", GitURL: moduleRepo},
			},
		},
	}

	if err := f.FetchAll(ctx, envs); err != nil {
		t.Fatalf("FetchAll error = %v", err)
	}

	// Both control and module repo should be cached
	for _, url := range []string{controlRepo, moduleRepo} {
		path := cm.RepoPath(url)
		if _, err := os.Stat(filepath.Join(path, "HEAD")); err != nil {
			t.Errorf("repo %s not cached", url)
		}
	}
}

func TestFetchAllParallelism(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repos")
	cacheDir := filepath.Join(tmpDir, "cache")

	// Create multiple repos to test parallel fetching
	var repos []string
	for i := 0; i < 5; i++ {
		repo := createTestRepo(t, repoDir, "module-"+string(rune('a'+i)))
		repos = append(repos, repo)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cm := cache.New(cacheDir, log)
	f := New(cm, 2, log) // Only 2 parallel workers
	ctx := context.Background()

	modules := make([]resolver.ResolvedModule, 0, len(repos))
	for i, r := range repos {
		modules = append(modules, resolver.ResolvedModule{
			Name:   "mod-" + string(rune('a'+i)),
			GitURL: r,
		})
	}

	envs := []resolver.ResolvedEnvironment{
		{Name: "prod", Modules: modules},
	}

	if err := f.FetchAll(ctx, envs); err != nil {
		t.Fatalf("FetchAll error = %v", err)
	}

	for _, r := range repos {
		path := cm.RepoPath(r)
		if _, err := os.Stat(filepath.Join(path, "HEAD")); err != nil {
			t.Errorf("repo not cached: %s", r)
		}
	}
}

func TestNewParallelMin(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	f := New(nil, 0, log)
	if f.parallel != 1 {
		t.Errorf("parallel = %d, want 1 (minimum)", f.parallel)
	}
}
