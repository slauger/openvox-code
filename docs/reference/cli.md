# CLI Reference

## Global Flags

| Flag | Description | Default |
|---|---|---|
| `--config` | Path to configuration file | `openvox-code.yaml` |
| `--cachedir` | Override cache directory | Value from config |
| `--environmentdir` | Override environment directory | Value from config |
| `--lockfile` | Path to lockfile | None |
| `--verbose` | Enable verbose output | `false` |
| `--quiet` | Suppress non-error output | `false` |
| `--parallel` | Max parallel Git operations | Number of CPUs |

## Commands

### `openvox-code sync`

Fetch and deploy all environments. This is the most common command — it runs the mirror phase followed by the deploy phase.

```bash
openvox-code sync --config openvox-code.yaml
```

Equivalent to running `mirror` followed by `deploy`.

| Flag | Description |
|---|---|
| `--lockfile` | Use a lockfile for pinned SHAs |
| `--clean` | Remove environments not in config |

### `openvox-code mirror`

Fetch and cache all Git repositories without deploying. Only performs network operations.

```bash
openvox-code mirror --config openvox-code.yaml
```

Use this to pre-populate the cache on a machine with network access before copying it to an air-gapped environment.

### `openvox-code deploy`

Deploy environments from the local cache. No network access required.

```bash
openvox-code deploy --config openvox-code.yaml
```

Fails if required refs are not present in the cache. Run `mirror` first or use a lockfile with known-good SHAs.

| Flag | Description |
|---|---|
| `--lockfile` | Use a lockfile for pinned SHAs |
| `--clean` | Remove environments not in config |

### `openvox-code diff`

Show what would change without making any modifications. Compares the current deployed state with what a `sync` would produce.

```bash
openvox-code diff --config openvox-code.yaml
```

Output includes:

- Environments to be added or removed
- Modules to be added, removed, or updated (with old and new SHAs)

### `openvox-code validate`

Validate the configuration file and check that all referenced Git refs are reachable.

```bash
openvox-code validate --config openvox-code.yaml
```

Checks:

- YAML syntax and schema validation
- All Git URLs are reachable (unless `--offline`)
- All refs (branches, tags, SHAs) exist in their repositories
- No duplicate module names within an environment
- Module set references are valid

| Flag | Description |
|---|---|
| `--offline` | Skip network checks, validate syntax only |

### `openvox-code build`

Build an OCI container image containing the deployed environments.

```bash
openvox-code build --config openvox-code.yaml --tag v1.0.0
```

| Flag | Description |
|---|---|
| `--tag` | Image tag |
| `--registry` | Override OCI registry URL |
| `--push` | Push image after building |

### `openvox-code lock`

Generate or update the lockfile by resolving all refs to concrete Git SHAs.

```bash
openvox-code lock --config openvox-code.yaml
```

Writes `openvox-code.lock` (or the path specified by `--lockfile`). The lockfile records the exact SHA for every module in every environment, enabling reproducible deploys.

| Flag | Description |
|---|---|
| `--lockfile` | Output path for lockfile |
