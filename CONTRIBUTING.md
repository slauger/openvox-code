# Contributing to openvox-code

## Getting Started

### Prerequisites

- Go 1.26+
- Docker or Podman (for container builds)
- golangci-lint
- Git

### Clone and Build

```bash
git clone https://github.com/slauger/openvox-code.git
cd openvox-code
make build
```

### Run Tests

```bash
make test           # Unit and integration tests with coverage
make lint           # golangci-lint
make vet            # go vet
make ci             # All checks (lint, vet, test, vulncheck)
```

## Development Workflow

1. Fork the repository
2. Create a feature branch from `main`
3. Implement your changes
4. Run `make ci` to verify all checks pass
5. Commit with a conventional commit message
6. Open a pull request against `main`

### Branching Convention

- `feat/<topic>` — new features
- `fix/<topic>` — bug fixes
- `ci/<topic>` — CI/CD changes
- `docs/<topic>` — documentation changes
- `refactor/<topic>` — code refactoring
- `test/<topic>` — test additions or changes

### Commit Messages

This project follows [Conventional Commits](https://www.conventionalcommits.org/). Commit messages are used for automated versioning via semantic-release.

| Prefix | Description | Version Bump |
|--------|-------------|--------------|
| `feat:` | New feature | Minor |
| `fix:` | Bug fix | Patch |
| `docs:` | Documentation | None |
| `ci:` | CI/CD changes | None |
| `chore:` | Maintenance | None |
| `refactor:` | Code refactoring | None |
| `test:` | Test changes | None |

Examples:

```
feat: add lockfile generation for reproducible deploys
fix: handle SSH URLs with non-standard ports in mirror rewriting
docs: add air-gapped deployment guide
```

## Project Structure

```
cmd/openvox-code/       CLI entrypoint and command definitions
internal/
  builder/              OCI image builder
  cache/                Bare clone cache management
  config/               YAML configuration parser
  deployer/             Atomic environment deployment
  fetcher/              Parallel Git fetch operations
  lock/                 Lockfile generation and parsing
  resolver/             Environment and module resolution
docs/                   MkDocs documentation
```

## Reporting Issues

Please report bugs and feature requests via [GitHub Issues](https://github.com/slauger/openvox-code/issues).

## License

MIT License — see [LICENSE](LICENSE) for details.
