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

	// Create test files mimicking environment structure
	if err := os.MkdirAll(filepath.Join(srcDir, "production", "modules", "stdlib"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "production", "modules", "stdlib", "init.pp"), []byte("class stdlib {}"), 0o600); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "test.tar")
	if err := createTarFromDir(srcDir, outPath); err != nil {
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

	// Files should be at root of archive (no /etc/puppetlabs prefix)
	hasProductionDir := false
	hasFile := false
	for _, e := range entries {
		if e == "production/" {
			hasProductionDir = true
		}
		if strings.Contains(e, "init.pp") {
			hasFile = true
			// Should NOT have a prefix like etc/puppetlabs/...
			if strings.HasPrefix(e, "etc/") {
				t.Errorf("file should be at root, not prefixed: %s", e)
			}
			// Should start with production/
			if !strings.HasPrefix(e, "production/") {
				t.Errorf("file should start with production/: %s", e)
			}
		}
	}

	if !hasProductionDir {
		t.Error("tar should contain production/ directory at root")
	}
	if !hasFile {
		t.Error("tar should contain init.pp file")
	}
}

func TestCreateTarFromEmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "empty.tar")

	if err := createTarFromDir(srcDir, outPath); err != nil {
		t.Fatalf("createTarFromDir error = %v", err)
	}

	// Empty dir should produce a valid but empty tar
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

	if count != 0 {
		t.Errorf("empty dir should produce empty tar, got %d entries", count)
	}
}
