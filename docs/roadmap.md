# Roadmap

## v0.1 — Core Sync

Minimum viable product: fetch Git repositories and deploy Puppet environments to disk.

- [x] YAML configuration parser (with Kubernetes-style envelope support)
- [x] Bare clone cache management
- [x] Parallel Git fetch
- [x] Branch discovery (dynamic environments) with `branchSelector` glob patterns
- [x] Static environment declarations
- [x] Module sets (shared across environments and sources)
- [x] Atomic deploy (temp dir + rename)
- [x] `sync`, `mirror`, `deploy` commands
- [x] `validate` command
- [x] `diff` command
- [x] Per-branch module files with glob support
- [x] Module `follow_branch` for environment-aware module resolution
- [x] Custom install paths (`target_dir`, `install_as`)
- [x] Source-level module sets and modules

## v0.2 — Lockfile + Offline Mode

Reproducible and air-gapped deployments.

- [x] Lockfile generation (`openvox-code lock`)
- [x] Deploy from lockfile with pinned SHAs
- [x] Full offline mode (`offline: true`)
- [x] Git mirror URL rewriting (`overrides.gitmirror`)
- [x] Per-host Git mirrors (`overrides.gitmirrors`)
- [x] Per-host Git credential resolution (`git.credentials`)
- [x] Retry with exponential backoff for transient network failures
- [x] Split configuration with `includes` (glob support)
- [ ] Cache portability (copy cache between machines)

## v0.3 — OCI Image Output

Package Puppet environments as container images for Kubernetes.

- [x] OCI image builder
- [x] Registry push support
- [x] `openvox-code build` command (with `-t` flag)
- [x] `openvox-code push` command
- [x] Multi-architecture image support
- [x] OCI registry auth config
- [ ] Integration with openvox-operator

## v0.4 — Webhook/Watch Mode + Metrics

Automation and observability.

- [ ] Webhook endpoint for Git push events
- [ ] File-watch mode for local development
- [ ] Prometheus metrics endpoint
- [ ] Structured JSON logging
- [ ] Health check endpoint
