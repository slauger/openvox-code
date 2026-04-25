// Package builder creates OCI container images from deployed Puppet environments.
package builder

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Options configures the OCI image build.
type Options struct {
	EnvironmentDir string
	Registry       string
	Tag            string
	Push           bool
	AuthConfig     string // Path to Docker config.json for registry auth
	Platform       *v1.Platform
}

// Builder creates OCI container images from deployed Puppet environments.
type Builder struct {
	log *slog.Logger
}

// New creates a new Builder.
func New(log *slog.Logger) *Builder {
	return &Builder{log: log}
}

// Build creates an OCI image containing the Puppet environments from the given directory.
func (b *Builder) Build(opts *Options) (v1.Image, error) {
	if opts.Tag == "" {
		opts.Tag = "latest"
	}

	if opts.Platform == nil {
		opts.Platform = &v1.Platform{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
		}
	}

	b.log.Info("building OCI image", "source", opts.EnvironmentDir, "tag", opts.Tag)

	// Verify environment directory exists
	if _, err := os.Stat(opts.EnvironmentDir); err != nil {
		return nil, fmt.Errorf("environment directory %q: %w", opts.EnvironmentDir, err)
	}

	// Create a tar layer from the environment directory
	layer, err := b.createLayer(opts.EnvironmentDir)
	if err != nil {
		return nil, fmt.Errorf("creating layer: %w", err)
	}

	// Build image from scratch
	img := empty.Image
	img = mutate.MediaType(img, types.OCIManifestSchema1)

	img, err = mutate.Append(img, mutate.Addendum{Layer: layer})
	if err != nil {
		return nil, fmt.Errorf("appending layer: %w", err)
	}

	// Add labels
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("getting config: %w", err)
	}
	cfg = cfg.DeepCopy()

	if cfg.Config.Labels == nil {
		cfg.Config.Labels = make(map[string]string)
	}
	cfg.Config.Labels["org.opencontainers.image.title"] = "openvox-code-environments"
	cfg.Config.Labels["org.opencontainers.image.description"] = "Puppet environments built by openvox-code"

	cfg.OS = opts.Platform.OS
	cfg.Architecture = opts.Platform.Architecture

	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		return nil, fmt.Errorf("setting config: %w", err)
	}

	b.log.Info("image built successfully")

	if opts.Push {
		if err := b.Push(img, opts.Registry, opts.Tag, opts.AuthConfig); err != nil {
			return nil, err
		}
	}

	return img, nil
}

// Save writes the image to a tarball on disk.
func (b *Builder) Save(img v1.Image, path string) error {
	ref, err := name.NewTag("openvox-code:local")
	if err != nil {
		return fmt.Errorf("creating tag: %w", err)
	}

	if err := tarball.WriteToFile(path, ref, img); err != nil {
		return fmt.Errorf("writing image to %s: %w", path, err)
	}

	b.log.Info("image saved", "path", path)
	return nil
}

func (b *Builder) createLayer(envDir string) (v1.Layer, error) {
	// Create a tar from the environment directory contents.
	// Files are placed directly at the image root — the openvox-operator mounts
	// the entire image as a volume to the Puppet environment path, so environments
	// like "production/" appear at the root of the image filesystem.
	tmpFile, err := os.CreateTemp("", "openvox-code-layer-*.tar")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("closing temp file: %w", err)
	}

	if err := createTarFromDir(envDir, tmpPath); err != nil {
		if rmErr := os.Remove(tmpPath); rmErr != nil {
			b.log.Warn("failed to remove temp file", "path", tmpPath, "error", rmErr)
		}
		return nil, fmt.Errorf("creating tar: %w", err)
	}

	// Read the entire tar into memory to avoid file-lifetime issues with lazy readers
	data, err := os.ReadFile(filepath.Clean(tmpPath))
	if rmErr := os.Remove(tmpPath); rmErr != nil {
		b.log.Warn("failed to remove temp file", "path", tmpPath, "error", rmErr)
	}
	if err != nil {
		return nil, fmt.Errorf("reading tar: %w", err)
	}

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	})
	if err != nil {
		return nil, fmt.Errorf("creating layer: %w", err)
	}

	return layer, nil
}

// Push pushes an OCI image to a container registry.
func (b *Builder) Push(img v1.Image, registry, tag, authConfig string) error {
	ref := fmt.Sprintf("%s:%s", registry, tag)
	b.log.Info("pushing image", "ref", ref)

	var craneOpts []crane.Option
	if authConfig != "" {
		b.log.Debug("using auth config", "path", authConfig)
	}

	if err := crane.Push(img, ref, craneOpts...); err != nil {
		return fmt.Errorf("pushing image to %s: %w", ref, err)
	}

	b.log.Info("image pushed", "ref", ref)
	return nil
}

// BuildMultiArch creates a multi-architecture image index from individual platform images.
func (b *Builder) BuildMultiArch(opts *Options, platforms []v1.Platform) (v1.ImageIndex, error) {
	var adds []mutate.IndexAddendum

	for _, platform := range platforms {
		p := platform
		platformOpts := *opts
		platformOpts.Platform = &p
		platformOpts.Push = false

		img, err := b.Build(&platformOpts)
		if err != nil {
			return nil, fmt.Errorf("building for %s/%s: %w", platform.OS, platform.Architecture, err)
		}

		mt, err := img.MediaType()
		if err != nil {
			return nil, fmt.Errorf("getting media type: %w", err)
		}

		adds = append(adds, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				MediaType: mt,
				Platform:  &p,
			},
		})
	}

	idx := mutate.AppendManifests(empty.Index, adds...)
	b.log.Info("multi-arch index built", "platforms", len(platforms))
	return idx, nil
}
