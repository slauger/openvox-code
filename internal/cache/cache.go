package cache

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles bare clone Git caches.
type Manager struct {
	baseDir string
	log     *slog.Logger
}

// New creates a new cache Manager.
func New(baseDir string, log *slog.Logger) *Manager {
	return &Manager{
		baseDir: baseDir,
		log:     log,
	}
}

// BaseDir returns the base cache directory.
func (m *Manager) BaseDir() string {
	return m.baseDir
}

// RepoPath returns the filesystem path for a cached bare clone of the given URL.
func (m *Manager) RepoPath(gitURL string) string {
	return filepath.Join(m.baseDir, "git", urlToDir(gitURL))
}

// EnsureClone ensures a bare clone exists for the given URL.
// If the clone already exists, it fetches updates; otherwise it creates a new bare clone.
func (m *Manager) EnsureClone(gitURL string) error {
	repoPath := m.RepoPath(gitURL)

	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); err == nil {
		m.log.Debug("updating bare clone", "url", gitURL, "path", repoPath)
		return m.fetch(repoPath)
	}

	m.log.Debug("creating bare clone", "url", gitURL, "path", repoPath)
	return m.clone(gitURL, repoPath)
}

// ResolveRef resolves a ref (branch, tag, or SHA) to a concrete SHA in the cache.
func (m *Manager) ResolveRef(gitURL, ref string) (string, error) {
	repoPath := m.RepoPath(gitURL)
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", ref)
	out, err := cmd.Output()
	if err != nil {
		// Try as remote ref
		cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "refs/heads/"+ref)
		out, err = cmd.Output()
		if err != nil {
			cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "refs/tags/"+ref)
			out, err = cmd.Output()
			if err != nil {
				return "", fmt.Errorf("resolving ref %q in %s: %w", ref, gitURL, err)
			}
		}
	}
	return strings.TrimSpace(string(out)), nil
}

// ListBranches lists all branches in a cached bare clone.
func (m *Manager) ListBranches(gitURL string) ([]string, error) {
	repoPath := m.RepoPath(gitURL)
	cmd := exec.Command("git", "-C", repoPath, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing branches for %s: %w", gitURL, err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// Checkout checks out files from a cached repo at the given SHA into the target directory.
func (m *Manager) Checkout(gitURL, sha, targetDir string) error {
	repoPath := m.RepoPath(gitURL)

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating target dir %s: %w", targetDir, err)
	}

	cmd := exec.Command("git", "-C", repoPath, "archive", "--format=tar", sha)
	tarCmd := exec.Command("tar", "-xf", "-", "-C", targetDir)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	tarCmd.Stdin = pipe

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting git archive: %w", err)
	}
	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("starting tar: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git archive for %s@%s: %w", gitURL, sha, err)
	}
	if err := tarCmd.Wait(); err != nil {
		return fmt.Errorf("tar extract: %w", err)
	}

	return nil
}

func (m *Manager) clone(gitURL, repoPath string) error {
	if err := os.MkdirAll(filepath.Dir(repoPath), 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	cmd := exec.Command("git", "clone", "--bare", "--mirror", gitURL, repoPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cloning %s: %w", gitURL, err)
	}
	return nil
}

func (m *Manager) fetch(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "fetch", "--prune", "--all")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fetching %s: %w", repoPath, err)
	}
	return nil
}

// urlToDir converts a Git URL to a safe directory name.
func urlToDir(gitURL string) string {
	// Handle SSH-style URLs
	if strings.Contains(gitURL, "@") && strings.Contains(gitURL, ":") && !strings.Contains(gitURL, "://") {
		gitURL = strings.Replace(gitURL, ":", "/", 1)
		gitURL = strings.SplitN(gitURL, "@", 2)[1]
	} else {
		parsed, err := url.Parse(gitURL)
		if err == nil {
			gitURL = parsed.Host + parsed.Path
		}
	}

	gitURL = strings.TrimSuffix(gitURL, ".git")

	replacer := strings.NewReplacer("/", "-", ":", "-", "@", "-")
	return replacer.Replace(gitURL) + ".git"
}
