# openvox-code

**openvox-code** is a fast, Git-native Puppet environment deployment tool written in Go.

It replaces [r10k](https://github.com/puppetlabs/r10k) and [g10k](https://github.com/xorpaul/g10k) with a simpler, more focused approach:

- **No Ruby** — single static Go binary, no runtime dependencies
- **No Puppetfile** — clean YAML configuration instead of Ruby DSL
- **Git-first** — bare clone caching, parallel fetches, atomic deploys
- **Offline-capable** — decouple mirroring from deployment
- **OCI output** — build container images with Puppet code for use with openvox-operator

## Why not r10k or g10k?

| | r10k | g10k | openvox-code |
|---|---|---|---|
| Language | Ruby | Go | Go |
| Config format | Puppetfile (Ruby DSL) | Puppetfile (Ruby DSL) | YAML |
| Puppet Forge | Yes | Yes | No (Git-only) |
| Parallel deploys | No | Yes | Yes |
| Offline mode | No | Partial | Yes |
| Lockfile | No | No | Yes |
| OCI image output | No | No | Yes |
| Atomic deploy | No | No | Yes |
| Status | Feature-frozen | Active | Active |

## Quick Start

```bash
openvox-code sync --config openvox-code.yaml
```

See [Configuration](concepts/configuration.md) for details.
