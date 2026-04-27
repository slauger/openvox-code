# Changelog

## Unreleased

### Features

- **Kubernetes-style configuration** with `apiVersion`/`kind`/`spec` envelope (`openvox.voxpupuli.org/v1alpha1`)
- **Branch discovery** with `branchSelector` glob pattern matching (`matchPatterns`/`excludePatterns`)
- **Control repo checkout** — branch content (manifests, hieradata, data) deployed as environment base
- **Per-branch module files** — `modules.yaml` read from inside the control repo with glob support
- **Module `follow_branch`** — try environment branch first, fall back to explicit `ref` or HEAD
- **Custom install paths** — `target_dir` and `install_as` for flexible module placement
- **Module exclude** — remove global modules per-environment via module file `exclude` list
- **Source-level modulesets** — global modules shared across all branches from a source
- **Lockfile** — `openvox-code lock` for reproducible deployments with pinned SHAs
- **Offline mode** — deploy from cache without network access (`offline: true`)
- **OCI image builder** — `openvox-code build -t registry:tag` (podman-style)
- **OCI push** — `openvox-code push registry:tag` as separate command
- **Multi-architecture images** — build image indexes for multiple platforms
- **OCI labels** — comprehensive `org.opencontainers.image.*` annotations
- **Per-host Git credentials** — different SSH keys and credential helpers per Git server
- **Host-specific Git mirrors** — `overrides.gitmirrors` map for per-host URL rewriting
- **Git retry** — automatic retry with exponential backoff (3 attempts) for transient network failures
- **Structured JSON logging** — `-o json` flag for machine-readable output
- **Standardized exit codes** — distinct codes for config, git, deploy, build, and validation errors
- **Parallel deployment** — both environments and modules deployed concurrently
- **`--environment` filter** — sync/deploy only specific environments
- **Puppetfile converter** — `openvox-code convert Puppetfile` for r10k/g10k migration
- **Config includes** — split configuration with glob patterns
- **Diff command** — show pending changes without deploying
- **Validate command** — syntax and consistency checking

### CI/CD

- Multi-OS test matrix (Linux + macOS)
- Container build validation in CI
- GoReleaser config check
- Coverage threshold enforcement (75%)
- golangci-lint v2 with zero issues
- govulncheck for dependency scanning

### Documentation

- Migration guide from r10k/g10k with Puppetfile mapping table
- Full configuration reference with K8s-style format
- CLI reference for all commands
- Architecture documentation with component descriptions
- Example configuration file

### Security

- Git URL sanitization in all logs and error messages
- Per-host credential isolation
- Input validation for Git refs (no shell-unsafe characters)
- Git subcommand allowlist to prevent command injection
- File path validation to prevent directory traversal
