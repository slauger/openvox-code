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

- [ ] Lockfile generation (`openvox-code lock`)
- [ ] Deploy from lockfile with pinned SHAs
- [ ] Full offline mode (`offline: true`)
- [ ] Git mirror URL rewriting (`overrides.gitmirror`)
- [ ] Cache portability (copy cache between machines)

## v0.3 — OCI Image Output

Package Puppet environments as container images for Kubernetes.

- [ ] OCI image builder
- [ ] Registry push support
- [ ] `openvox-code build` command
- [ ] Integration with openvox-operator
- [ ] Multi-architecture image support

## v0.4 — Webhook/Watch Mode + Metrics

Automation and observability.

- [ ] Webhook endpoint for Git push events
- [ ] File-watch mode for local development
- [ ] Prometheus metrics endpoint
- [ ] Structured JSON logging
- [ ] Health check endpoint
