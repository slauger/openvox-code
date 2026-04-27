package config

import (
	"testing"
)

func TestRewriteGitURL(t *testing.T) {
	tests := []struct {
		name       string
		gitURL     string
		mirrorBase string
		want       string
	}{
		{
			name:       "HTTPS URL",
			gitURL:     "https://github.com/puppetlabs/puppetlabs-stdlib.git",
			mirrorBase: "https://mirror.example.com",
			want:       "https://mirror.example.com/puppetlabs/puppetlabs-stdlib.git",
		},
		{
			name:       "HTTPS URL with trailing slash",
			gitURL:     "https://github.com/puppetlabs/puppetlabs-stdlib.git",
			mirrorBase: "https://mirror.example.com/",
			want:       "https://mirror.example.com/puppetlabs/puppetlabs-stdlib.git",
		},
		{
			name:       "SSH URL",
			gitURL:     "git@github.com:puppetlabs/puppetlabs-stdlib.git",
			mirrorBase: "https://mirror.example.com",
			want:       "https://mirror.example.com/puppetlabs/puppetlabs-stdlib.git",
		},
		{
			name:       "mirror with path",
			gitURL:     "https://github.com/puppetlabs/puppetlabs-stdlib.git",
			mirrorBase: "https://mirror.example.com/git",
			want:       "https://mirror.example.com/git/puppetlabs/puppetlabs-stdlib.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteGitURL(tt.gitURL, tt.mirrorBase)
			if got != tt.want {
				t.Errorf("rewriteGitURL(%q, %q) = %q, want %q", tt.gitURL, tt.mirrorBase, got, tt.want)
			}
		})
	}
}

func TestHostMatches(t *testing.T) {
	tests := []struct {
		name   string
		gitURL string
		host   string
		want   bool
	}{
		{name: "HTTPS match", gitURL: "https://github.com/owner/repo.git", host: "github.com", want: true},
		{name: "HTTPS no match", gitURL: "https://github.com/owner/repo.git", host: "gitlab.com", want: false},
		{name: "SSH match", gitURL: "git@github.com:owner/repo.git", host: "github.com", want: true},
		{name: "SSH no match", gitURL: "git@github.com:owner/repo.git", host: "gitlab.com", want: false},
		{name: "HTTPS with port", gitURL: "https://git.example.com:8443/repo.git", host: "git.example.com", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hostMatches(tt.gitURL, tt.host)
			if got != tt.want {
				t.Errorf("hostMatches(%q, %q) = %v, want %v", tt.gitURL, tt.host, got, tt.want)
			}
		})
	}
}

func TestResolveGitURLWithMirrorMap(t *testing.T) {
	cfg := &Config{
		Overrides: Overrides{
			GitMirrors: map[string]string{
				"github.com": "https://github-mirror.internal",
				"gitlab.com": "https://gitlab-mirror.internal",
			},
			GitMirror: "https://fallback-mirror.internal",
		},
	}

	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "github matched by mirror map",
			url:  "https://github.com/puppetlabs/stdlib.git",
			want: "https://github-mirror.internal/puppetlabs/stdlib.git",
		},
		{
			name: "gitlab matched by mirror map",
			url:  "https://gitlab.com/org/module.git",
			want: "https://gitlab-mirror.internal/org/module.git",
		},
		{
			name: "unknown host falls back to global mirror",
			url:  "https://bitbucket.org/team/repo.git",
			want: "https://fallback-mirror.internal/team/repo.git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ResolveGitURL(tt.url)
			if got != tt.want {
				t.Errorf("ResolveGitURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "no credentials", url: "https://github.com/org/repo.git", want: "https://github.com/org/repo.git"},
		{name: "token in URL", url: "https://ghp_abc123@github.com/org/repo.git", want: "https://***@github.com/org/repo.git"},
		{name: "user and auth", url: "https://user:" + "x-token" + "@gitlab.com/org/repo.git", want: "https://***@gitlab.com/org/repo.git"},
		{name: "SSH URL unchanged", url: "git@github.com:org/repo.git", want: "git@github.com:org/repo.git"},
		{name: "empty", url: "", want: ""},
		{name: "plain path", url: "/local/repo.git", want: "/local/repo.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeURL(tt.url)
			if got != tt.want {
				t.Errorf("SanitizeURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
