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

func TestBaseDir(t *testing.T) {
	m := New("/var/cache/openvox-code", nil)
	if got := m.BaseDir(); got != "/var/cache/openvox-code" {
		t.Errorf("BaseDir() = %q, want %q", got, "/var/cache/openvox-code")
	}
}

func TestValidateGitArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "allowed subcommand", args: []string{"clone", "--bare", "url"}, wantErr: false},
		{name: "fetch with prune", args: []string{"-C", "/path", "fetch", "--prune"}, wantErr: false},
		{name: "rev-parse", args: []string{"-C", "/path", "rev-parse", "--verify", "HEAD"}, wantErr: false},
		{name: "for-each-ref", args: []string{"-C", "/path", "for-each-ref"}, wantErr: false},
		{name: "archive", args: []string{"-C", "/path", "archive", "--format=tar", "abc123"}, wantErr: false},
		{name: "disallowed subcommand", args: []string{"push", "origin"}, wantErr: true},
		{name: "disallowed rm", args: []string{"-C", "/path", "rm", "file"}, wantErr: true},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "only flags", args: []string{"-C", "/path"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitArgs(tt.args)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
