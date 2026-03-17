# 🦊 openvox-code

> **🚧 Status: Concept Phase** — This project is in the early concept and design phase. Nothing is implemented yet. We are currently collecting ideas and defining the scope. Feedback, suggestions, and ideas are very welcome — feel free to open an [issue](https://github.com/slauger/openvox-code/issues) or start a [discussion](https://github.com/slauger/openvox-code/discussions).

Fast, Git-native Puppet environment deployment tool written in Go.

openvox-code replaces [r10k](https://github.com/puppetlabs/r10k) and [g10k](https://github.com/xorpaul/g10k) with a simpler, more focused approach:

- 🏗️ **No Ruby** — single static Go binary, no runtime dependencies
- 📝 **No Puppetfile** — clean YAML configuration instead of Ruby DSL
- 🔀 **Git-first** — bare clone caching, parallel fetches, atomic deploys
- 📴 **Offline-capable** — decouple mirroring from deployment
- 📦 **OCI output** — build container images with Puppet code for use with [openvox-operator](https://github.com/slauger/openvox-operator)

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

## Documentation

Full documentation is available at [slauger.github.io/openvox-code](https://slauger.github.io/openvox-code).

- [Architecture](https://slauger.github.io/openvox-code/concepts/architecture/)
- [Configuration Format](https://slauger.github.io/openvox-code/concepts/configuration/)
- [Environment Management](https://slauger.github.io/openvox-code/concepts/environments/)
- [Module Caching](https://slauger.github.io/openvox-code/concepts/caching/)
- [CLI Reference](https://slauger.github.io/openvox-code/reference/cli/)
- [Roadmap](https://slauger.github.io/openvox-code/roadmap/)

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
