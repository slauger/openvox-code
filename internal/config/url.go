package config

import (
	"net/url"
	"strings"
)

// rewriteGitURL rewrites a Git URL to use the given mirror base URL,
// preserving the path component.
func rewriteGitURL(gitURL, mirrorBase string) string {
	// Handle SSH-style URLs first (git@github.com:owner/repo.git)
	if isSSHURL(gitURL) {
		parts := strings.SplitN(gitURL, ":", 2)
		if len(parts) == 2 {
			path := parts[1]
			return strings.TrimRight(mirrorBase, "/") + "/" + path
		}
		return gitURL
	}

	parsed, err := url.Parse(gitURL)
	if err != nil {
		return gitURL
	}

	mirror, err := url.Parse(mirrorBase)
	if err != nil {
		return gitURL
	}

	mirror.Path = strings.TrimRight(mirror.Path, "/") + parsed.Path
	return mirror.String()
}

// isSSHURL detects SSH-style Git URLs like git@github.com:owner/repo.git
func isSSHURL(u string) bool {
	// SSH URLs have @ before the host and : before the path, but no ://
	if strings.Contains(u, "://") {
		return false
	}
	atIdx := strings.Index(u, "@")
	colonIdx := strings.Index(u, ":")
	return atIdx >= 0 && colonIdx > atIdx
}
