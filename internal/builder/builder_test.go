package builder

import (
	"archive/tar"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createTestEnvironments(t *testing.T, dir string) {
	t.Helper()

	envs := map[string]map[string]string{
		"production/modules/stdlib": {"init.pp": "class stdlib {}"},
		"production/modules/apache": {"init.pp": "class apache {}"},
		"staging/modules/stdlib":    {"init.pp": "class stdlib {}"},
	}

	for path, files := range envs {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(fullPath, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(fullPath, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestBuild(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	createTestEnvironments(t, envDir)

	b := New(testLogger())
	img, err := b.Build(Options{
		EnvironmentDir: envDir,
		Tag:            "v1.0.0",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Verify image has layers
	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("Layers() error = %v", err)
	}
	if len(layers) != 1 {
		t.Errorf("got %d layers, want 1", len(layers))
	}

	// Verify config labels
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile() error = %v", err)
	}
	if cfg.Config.Labels["org.opencontainers.image.title"] != "openvox-code-environments" {
		t.Error("missing or wrong title label")
	}
}

func TestBuildWithPlatform(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	createTestEnvironments(t, envDir)

	b := New(testLogger())
	img, err := b.Build(Options{
		EnvironmentDir: envDir,
		Platform: &v1.Platform{
			OS:           "linux",
			Architecture: "arm64",
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile() error = %v", err)
	}
	if cfg.OS != "linux" {
		t.Errorf("OS = %q, want linux", cfg.OS)
	}
	if cfg.Architecture != "arm64" {
		t.Errorf("Architecture = %q, want arm64", cfg.Architecture)
	}
}

func TestBuildInvalidDir(t *testing.T) {
	b := New(testLogger())
	_, err := b.Build(Options{
		EnvironmentDir: "/nonexistent/path",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	createTestEnvironments(t, envDir)

	b := New(testLogger())
	img, err := b.Build(Options{
		EnvironmentDir: envDir,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	outPath := filepath.Join(dir, "image.tar")
	if err := b.Save(img, outPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists and is non-empty
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}

func TestBuildLayerContents(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	createTestEnvironments(t, envDir)

	b := New(testLogger())
	img, err := b.Build(Options{
		EnvironmentDir: envDir,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	layers, _ := img.Layers()
	if len(layers) == 0 {
		t.Fatal("no layers")
	}

	rc, err := layers[0].Uncompressed()
	if err != nil {
		t.Fatalf("Uncompressed() error = %v", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	var foundFiles []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		foundFiles = append(foundFiles, hdr.Name)
	}

	// Check for expected paths
	expectedPrefixes := []string{
		"etc/puppetlabs/code/environments/production/",
		"etc/puppetlabs/code/environments/staging/",
	}

	for _, prefix := range expectedPrefixes {
		found := false
		for _, f := range foundFiles {
			if strings.HasPrefix(f, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected path starting with %q not found in tar", prefix)
		}
	}
}

func TestBuildMultiArch(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	createTestEnvironments(t, envDir)

	b := New(testLogger())
	platforms := []v1.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
	}

	idx, err := b.BuildMultiArch(Options{
		EnvironmentDir: envDir,
		Tag:            "v1.0.0",
	}, platforms)
	if err != nil {
		t.Fatalf("BuildMultiArch() error = %v", err)
	}

	manifest, err := idx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest() error = %v", err)
	}
	if len(manifest.Manifests) != 2 {
		t.Errorf("got %d manifests, want 2", len(manifest.Manifests))
	}
}

func TestBuildDefaultTag(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	createTestEnvironments(t, envDir)

	b := New(testLogger())
	// Empty tag should default to "latest"
	_, err := b.Build(Options{
		EnvironmentDir: envDir,
	})
	if err != nil {
		t.Fatalf("Build() with empty tag error = %v", err)
	}
}
