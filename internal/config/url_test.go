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
