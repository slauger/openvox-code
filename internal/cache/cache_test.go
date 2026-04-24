package cache

import (
	"testing"
)

func TestURLToDir(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "HTTPS URL",
			url:  "https://github.com/puppetlabs/puppetlabs-stdlib.git",
			want: "github.com-puppetlabs-puppetlabs-stdlib.git",
		},
		{
			name: "SSH URL",
			url:  "git@github.com:puppetlabs/puppetlabs-stdlib.git",
			want: "github.com-puppetlabs-puppetlabs-stdlib.git",
		},
		{
			name: "HTTPS URL without .git",
			url:  "https://github.com/puppetlabs/puppetlabs-stdlib",
			want: "github.com-puppetlabs-puppetlabs-stdlib.git",
		},
		{
			name: "URL with port",
			url:  "https://git.example.com:8443/repo/module.git",
			want: "git.example.com-8443-repo-module.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := urlToDir(tt.url)
			if got != tt.want {
				t.Errorf("urlToDir(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestRepoPath(t *testing.T) {
	m := New("/var/cache/openvox-code", nil)
	got := m.RepoPath("https://github.com/puppetlabs/puppetlabs-stdlib.git")
	want := "/var/cache/openvox-code/git/github.com-puppetlabs-puppetlabs-stdlib.git"
	if got != want {
		t.Errorf("RepoPath() = %q, want %q", got, want)
	}
}
