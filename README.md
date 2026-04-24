# 🦊 openvox-code

[![CI](https://github.com/slauger/openvox-code/actions/workflows/ci.yaml/badge.svg)](https://github.com/slauger/openvox-code/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/slauger/openvox-code)](https://goreportcard.com/report/github.com/slauger/openvox-code)

Fast, Git-native Puppet environment deployment tool written in Go.

openvox-code replaces [r10k](https://github.com/puppetlabs/r10k) and [g10k](https://github.com/xorpaul/g10k) with a simpler, more focused approach:

- 🏗️ **No Ruby** — single static Go binary, no runtime dependencies
- 📝 **No Puppetfile** — clean YAML configuration instead of Ruby DSL
- 🔀 **Git-first** — bare clone caching, parallel fetches, atomic deploys
- 📴 **Offline-capable** — decouple mirroring from deployment
- 📦 **OCI output** — build container images with Puppet code for use with [openvox-operator](https://github.com/slauger/openvox-operator)
- 🚀 **CI/CD-first** — build artifacts once in CI, deploy immutable images everywhere (no Git access needed on target nodes)

## Why not r10k or g10k?

Traditional tools like r10k and g10k run **on each Puppet server**, pulling code directly from Git. This means:

- Every server independently fetches the same repos — N servers = N times the Git load
- Code deployments are non-atomic and can leave servers in inconsistent states
- Git servers are hit every time any Puppet server needs a code update

openvox-code takes a different approach: it runs **once in CI/CD**, builds an immutable OCI image containing all Puppet environments, and pushes it to a container registry. Puppet servers pull the finished image — no Git access required on target nodes, guaranteed consistency, and the Git server is only hit once per build.

## Install

### Binary

Download the latest release from [GitHub Releases](https://github.com/slauger/openvox-code/releases).

### Go

```bash
go install github.com/slauger/openvox-code/cmd/openvox-code@latest
```

### Container

```bash
docker pull ghcr.io/slauger/openvox-code:latest
```

## Quick Start

```bash
# Fetch and deploy all environments
openvox-code sync --config openvox-code.yaml
```

Minimal configuration (`openvox-code.yaml`):

```yaml
cachedir: /var/cache/openvox-code
environmentdir: /etc/puppetlabs/code/environments

sources:
  - url: https://github.com/example/control-repo.git
    branches: all
```

## Commands

```
openvox-code sync       Fetch and deploy all environments
openvox-code mirror     Only fetch/cache repos, no deploy
openvox-code deploy     Deploy from cache (no network)
openvox-code diff       Show what would change
openvox-code validate   Validate config and check all refs reachable
openvox-code build      Build OCI image with environments
openvox-code lock       Generate/update lockfile
```

## CI/CD Usage

Build and push an OCI image in your CI pipeline:

```bash
# Sync environments locally
openvox-code sync --config openvox-code.yaml

# Build and push OCI image
openvox-code build --config openvox-code.yaml --registry ghcr.io/example/puppet-envs --tag v1.0.0 --push
```

## Documentation

Full documentation is available at [slauger.github.io/openvox-code](https://slauger.github.io/openvox-code).

- [Architecture](https://slauger.github.io/openvox-code/concepts/architecture/)
- [Configuration Format](https://slauger.github.io/openvox-code/concepts/configuration/)
- [Environment Management](https://slauger.github.io/openvox-code/concepts/environments/)
- [Module Caching](https://slauger.github.io/openvox-code/concepts/caching/)
- [CLI Reference](https://slauger.github.io/openvox-code/reference/cli/)
- [Roadmap](https://slauger.github.io/openvox-code/roadmap/)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

MIT License — see [LICENSE](LICENSE) for details.
