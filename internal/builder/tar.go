package builder

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// createTarFromDir creates a tar archive from a directory, prefixing all entries with targetPrefix.
func createTarFromDir(srcDir, targetPrefix, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	// Add directory entries for the prefix path
	parts := strings.Split(targetPrefix, "/")
	for i := range parts {
		dirPath := strings.Join(parts[:i+1], "/") + "/"
		if err := tw.WriteHeader(&tar.Header{
			Name:     dirPath,
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		}); err != nil {
			return fmt.Errorf("writing dir header %s: %w", dirPath, err)
		}
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("getting relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		tarPath := filepath.Join(targetPrefix, relPath)
		// Normalize to forward slashes for tar
		tarPath = filepath.ToSlash(tarPath)

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("creating header for %s: %w", relPath, err)
		}
		header.Name = tarPath

		if info.IsDir() {
			header.Name += "/"
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing header for %s: %w", tarPath, err)
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		defer file.Close()

		if _, err := io.Copy(tw, file); err != nil {
			return fmt.Errorf("writing %s: %w", tarPath, err)
		}

		return nil
	})
}
