package builder

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// createTarFromDir creates a tar archive from a directory.
// Files are placed relative to the archive root (no prefix).
func createTarFromDir(srcDir, outputPath string) (err error) {
	cleanedOutput := filepath.Clean(outputPath)
	f, err := os.Create(cleanedOutput)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	tw := tar.NewWriter(f)
	defer func() { err = errors.Join(err, tw.Close()) }()

	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("resolving source dir: %w", err)
	}

	return filepath.WalkDir(absSrcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(absSrcDir, path)
		if err != nil {
			return fmt.Errorf("getting relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		// Validate the path stays within the source directory
		if _, pathErr := filepath.Rel(absSrcDir, filepath.Clean(path)); pathErr != nil {
			return fmt.Errorf("path escapes source directory: %w", pathErr)
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("getting file info for %s: %w", relPath, err)
		}

		// Use relative path directly (no prefix) — files at root of archive
		tarPath := filepath.ToSlash(relPath)

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

		if err := addFileToTar(tw, path); err != nil {
			return fmt.Errorf("adding %s to tar: %w", tarPath, err)
		}

		return nil
	})
}

// addFileToTar reads a file and writes its contents to the tar writer.
func addFileToTar(tw *tar.Writer, path string) (err error) {
	file, err := os.Open(path) // #nosec G304 -- path is validated by WalkDir within a known directory
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	if _, err := io.Copy(tw, file); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
