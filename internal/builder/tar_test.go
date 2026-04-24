package builder

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateTarFromDir(t *testing.T) {
	srcDir := t.TempDir()

	// Create test files
	if err := os.MkdirAll(filepath.Join(srcDir, "production", "modules", "stdlib"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "production", "modules", "stdlib", "init.pp"), []byte("class stdlib {}"), 0o600); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "test.tar")
	if err := createTarFromDir(srcDir, "puppet/environments", outPath); err != nil {
		t.Fatalf("createTarFromDir error = %v", err)
	}

	// Read and verify tar contents
	f, err := os.Open(filepath.Clean(outPath))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("closing tar file: %v", err)
		}
	}()

	tr := tar.NewReader(f)
	var entries []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		entries = append(entries, hdr.Name)
	}

	// Should have prefix dirs and the actual files
	hasPrefix := false
	hasFile := false
	for _, e := range entries {
		if e == "puppet/" || e == "puppet/environments/" {
			hasPrefix = true
		}
		if strings.Contains(e, "init.pp") {
			hasFile = true
		}
	}

	if !hasPrefix {
		t.Error("tar should contain prefix directory entries")
	}
	if !hasFile {
		t.Error("tar should contain init.pp file")
	}
}

func TestCreateTarFromEmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "empty.tar")

	if err := createTarFromDir(srcDir, "prefix", outPath); err != nil {
		t.Fatalf("createTarFromDir error = %v", err)
	}

	// Should still create a valid tar with just the prefix dir
	f, err := os.Open(filepath.Clean(outPath))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("closing tar file: %v", err)
		}
	}()

	tr := tar.NewReader(f)
	count := 0
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		count++
	}

	if count == 0 {
		t.Error("expected at least prefix directory entries")
	}
}
