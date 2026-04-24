# Roadmap

## v0.1 — Core Sync

Minimum viable product: fetch Git repositories and deploy Puppet environments to disk.

- [x] YAML configuration parser
- [x] Bare clone cache management
- [x] Parallel Git fetch
- [x] Branch discovery (dynamic environments)
- [x] Static environment declarations
- [x] Module sets
- [x] Atomic deploy (temp dir + rename)
- [x] `sync`, `mirror`, `deploy` commands
- [x] `validate` command
- [x] `diff` command

## v0.2 — Lockfile + Offline Mode

Reproducible and air-gapped deployments.

- [x] Lockfile generation (`openvox-code lock`)
- [x] Deploy from lockfile with pinned SHAs
- [x] Full offline mode (`offline: true`)
- [x] Git mirror URL rewriting (`overrides.gitmirror`)
- [ ] Cache portability (copy cache between machines)

## v0.3 — OCI Image Output

Package Puppet environments as container images for Kubernetes.

- [x] OCI image builder
- [x] Registry push support
- [x] `openvox-code build` command
- [x] Multi-architecture image support
- [ ] Integration with openvox-operator

## v0.4 — Webhook/Watch Mode + Metrics

Automation and observability.

- [ ] Webhook endpoint for Git push events
- [ ] File-watch mode for local development
- [ ] Prometheus metrics endpoint
- [ ] Structured JSON logging
- [ ] Health check endpoint
