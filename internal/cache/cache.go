// Package cache manages bare-clone Git caches for efficient repository storage.
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// allowedGitSubcommands is the set of git subcommands that the cache manager
// is permitted to invoke. This mitigates gosec G204 by restricting what
// can be executed via variable arguments.
var allowedGitSubcommands = map[string]bool{
	"clone":        true,
	"fetch":        true,
	"rev-parse":    true,
	"for-each-ref": true,
	"archive":      true,
	"-C":           true,
}

// CredentialResolver returns credentials for a given Git URL.
type CredentialResolver func(gitURL string) (sshKey, knownHosts, credentialHelper string)

// Manager handles bare clone Git caches.
type Manager struct {
	baseDir            string
	credentialResolver CredentialResolver
	log                *slog.Logger
}

// New creates a new cache Manager.
func New(baseDir string, log *slog.Logger) *Manager {
	return &Manager{
		baseDir: baseDir,
		log:     log,
	}
}

// SetCredentialResolver configures per-URL credential resolution.
func (m *Manager) SetCredentialResolver(resolver CredentialResolver) {
	m.credentialResolver = resolver
}

// BaseDir returns the base cache directory.
func (m *Manager) BaseDir() string {
	return m.baseDir
}

// RepoPath returns the filesystem path for a cached bare clone of the given URL.
func (m *Manager) RepoPath(gitURL string) string {
	return filepath.Join(m.baseDir, "git", urlToDir(gitURL))
}

// git executes a git command with the given arguments using the provided context.
// It validates that only known-safe git subcommands are used.
func (m *Manager) git(ctx context.Context, args ...string) ([]byte, error) {
	if err := validateGitArgs(args); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- args validated above
	return cmd.Output()
}

// gitWithAuth executes a git command with URL-specific credentials.
func (m *Manager) gitWithAuth(ctx context.Context, gitURL string, args ...string) error {
	if err := validateGitArgs(args); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- args validated above
	cmd.Stderr = os.Stderr
	m.applyAuthForURL(cmd, gitURL)
	return cmd.Run()
}

// applyAuth sets environment variables on a git command for SSH key and credential helper auth.
// If gitURL is provided and a credential resolver is set, per-URL credentials are used.
func (m *Manager) applyAuthForURL(cmd *exec.Cmd, gitURL string) {
	if m.credentialResolver == nil {
		return
	}

	sshKey, knownHosts, credHelper := m.credentialResolver(gitURL)
	if sshKey == "" && credHelper == "" {
		return
	}

	cmd.Env = append(os.Environ(), cmd.Env...)

	if sshKey != "" {
		sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", sshKey)
		if knownHosts != "" {
			sshCmd += fmt.Sprintf(" -o UserKnownHostsFile=%s", knownHosts)
		}
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND="+sshCmd)
	}

	if credHelper != "" {
		cmd.Env = append(cmd.Env,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=credential.helper",
			"GIT_CONFIG_VALUE_0="+credHelper,
		)
	}
}

// validateGitArgs checks that the git subcommand is in the allowed set.
// It correctly handles flags like -C which take a path argument.
func validateGitArgs(args []string) error {
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		// -C takes a path argument; skip both -C and its value
		if arg == "-C" {
			skipNext = true
			continue
		}
		// Skip other flags (e.g. --format=..., --verify, --prune)
		if strings.HasPrefix(arg, "-") {
			continue
		}
		// First positional arg is the subcommand
		if !allowedGitSubcommands[arg] {
			return fmt.Errorf("disallowed git subcommand: %q", arg)
		}
		return nil
	}
	return fmt.Errorf("no git subcommand found in args")
}

// MaxRetries is the number of times to retry transient Git network operations.
const MaxRetries = 3

// EnsureClone ensures a bare clone exists for the given URL.
// If the clone already exists, it fetches updates; otherwise it creates a new bare clone.
// Transient network errors are retried with exponential backoff.
func (m *Manager) EnsureClone(ctx context.Context, gitURL string) error {
	repoPath := m.RepoPath(gitURL)

	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); err == nil {
		m.log.Debug("updating bare clone", "url", gitURL, "path", repoPath)
		return m.retryNetworkOp(ctx, "fetch", gitURL, func() error {
			return m.fetch(ctx, gitURL, repoPath)
		})
	}

	m.log.Debug("creating bare clone", "url", gitURL, "path", repoPath)
	return m.retryNetworkOp(ctx, "clone", gitURL, func() error {
		return m.clone(ctx, gitURL, repoPath)
	})
}

// retryNetworkOp retries a Git network operation with exponential backoff.
func (m *Manager) retryNetworkOp(ctx context.Context, op, gitURL string, fn func() error) error {
	var lastErr error
	for attempt := range MaxRetries {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt < MaxRetries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second // 1s, 2s, 4s
			m.log.Warn("git operation failed, retrying",
				"op", op, "url", gitURL,
				"attempt", attempt+1, "max", MaxRetries,
				"backoff", backoff, "error", lastErr)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return fmt.Errorf("git %s failed after %d attempts: %w", op, MaxRetries, lastErr)
}

// ResolveRef resolves a ref (branch, tag, or SHA) to a concrete SHA in the cache.
func (m *Manager) ResolveRef(ctx context.Context, gitURL, ref string) (string, error) {
	repoPath := m.RepoPath(gitURL)

	// Validate ref does not contain shell-unsafe characters
	if strings.ContainsAny(ref, ";&|`$\\") {
		return "", fmt.Errorf("invalid ref %q: contains disallowed characters", ref)
	}

	out, err := m.git(ctx, "-C", repoPath, "rev-parse", "--verify", ref)
	if err != nil {
		// Try as remote ref
		out, err = m.git(ctx, "-C", repoPath, "rev-parse", "--verify", "refs/heads/"+ref)
		if err != nil {
			out, err = m.git(ctx, "-C", repoPath, "rev-parse", "--verify", "refs/tags/"+ref)
			if err != nil {
				return "", fmt.Errorf("resolving ref %q in %s: %w", ref, gitURL, err)
			}
		}
	}
	return strings.TrimSpace(string(out)), nil
}

// ListBranches lists all branches in a cached bare clone.
func (m *Manager) ListBranches(ctx context.Context, gitURL string) ([]string, error) {
	repoPath := m.RepoPath(gitURL)
	out, err := m.git(ctx, "-C", repoPath, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
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
func (m *Manager) Checkout(ctx context.Context, gitURL, sha, targetDir string) error {
	repoPath := m.RepoPath(gitURL)

	// Validate sha does not contain shell-unsafe characters
	if strings.ContainsAny(sha, ";&|`$\\") {
		return fmt.Errorf("invalid sha %q: contains disallowed characters", sha)
	}

	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return fmt.Errorf("creating target dir %s: %w", targetDir, err)
	}

	archiveCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "archive", "--format=tar", sha) // #nosec G204 -- sha validated above
	tarCmd := exec.CommandContext(ctx, "tar", "-xf", "-", "-C", targetDir)                        // #nosec G204 -- targetDir is an internal path

	pipe, err := archiveCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	tarCmd.Stdin = pipe

	if err := archiveCmd.Start(); err != nil {
		return fmt.Errorf("starting git archive: %w", err)
	}
	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("starting tar: %w", err)
	}
	if err := archiveCmd.Wait(); err != nil {
		return fmt.Errorf("git archive for %s@%s: %w", gitURL, sha, err)
	}
	if err := tarCmd.Wait(); err != nil {
		return fmt.Errorf("tar extract: %w", err)
	}

	return nil
}

func (m *Manager) clone(ctx context.Context, gitURL, repoPath string) error {
	if err := os.MkdirAll(filepath.Dir(repoPath), 0o750); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	if err := m.gitWithAuth(ctx, gitURL, "clone", "--bare", "--mirror", gitURL, repoPath); err != nil {
		return fmt.Errorf("cloning %s: %w", gitURL, err)
	}
	return nil
}

func (m *Manager) fetch(ctx context.Context, gitURL, repoPath string) error {
	if err := m.gitWithAuth(ctx, gitURL, "-C", repoPath, "fetch", "--prune", "--all"); err != nil {
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
