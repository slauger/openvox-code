package cache

import (
	"context"
	"errors"
	"log/slog"
	"os"
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

func testCacheLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestRetryNetworkOpSuccess(t *testing.T) {
	m := New("/tmp/test", testCacheLogger())
	calls := 0
	err := m.retryNetworkOp(context.Background(), "test", "url", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryNetworkOpRetryThenSuccess(t *testing.T) {
	m := New("/tmp/test", testCacheLogger())
	calls := 0
	err := m.retryNetworkOp(context.Background(), "test", "url", func() error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryNetworkOpAllFail(t *testing.T) {
	m := New("/tmp/test", testCacheLogger())
	calls := 0
	err := m.retryNetworkOp(context.Background(), "clone", "https://example.com/repo.git", func() error {
		calls++
		return errors.New("network error")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != MaxRetries {
		t.Errorf("expected %d calls, got %d", MaxRetries, calls)
	}
}

func TestRetryNetworkOpContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := New("/tmp/test", testCacheLogger())
	calls := 0
	cancel() // Cancel immediately
	err := m.retryNetworkOp(ctx, "fetch", "url", func() error {
		calls++
		return errors.New("will fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls < 1 {
		t.Errorf("expected at least 1 call, got %d", calls)
	}
}
