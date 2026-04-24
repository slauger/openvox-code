package builder

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// createTarFromDir creates a tar archive from a directory, prefixing all entries with targetPrefix.
func createTarFromDir(srcDir, targetPrefix, outputPath string) (err error) {
	cleanedOutput := filepath.Clean(outputPath)
	f, err := os.Create(cleanedOutput)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	tw := tar.NewWriter(f)
	defer func() { err = errors.Join(err, tw.Close()) }()

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
		if _, err := filepath.Rel(absSrcDir, filepath.Clean(path)); err != nil {
			return fmt.Errorf("path escapes source directory: %w", err)
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("getting file info for %s: %w", relPath, err)
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
