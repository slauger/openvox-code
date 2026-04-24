package config

import (
	"net/url"
	"strings"
)

// rewriteGitURL rewrites a Git URL to use the given mirror base URL,
// preserving the path component.
func rewriteGitURL(gitURL, mirrorBase string) string {
	parsed, err := url.Parse(gitURL)
	if err != nil {
		return gitURL
	}

	mirror, err := url.Parse(mirrorBase)
	if err != nil {
		return gitURL
	}

	// For SSH-style URLs (git@github.com:owner/repo.git)
	if strings.Contains(gitURL, "@") && strings.Contains(gitURL, ":") && !strings.Contains(gitURL, "://") {
		parts := strings.SplitN(gitURL, ":", 2)
		if len(parts) == 2 {
			path := parts[1]
			return strings.TrimRight(mirrorBase, "/") + "/" + path
		}
	}

	// For standard URLs
	mirror.Path = strings.TrimRight(mirror.Path, "/") + parsed.Path
	return mirror.String()
}
