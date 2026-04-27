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

Fetch and deploy all environments. This is the most common command -- it runs the mirror phase followed by the deploy phase.

```bash
openvox-code sync --config openvox-code.yaml
```

Equivalent to running `mirror` followed by `deploy`.

| Flag | Description |
|---|---|
| `--clean` | Remove environments not in config |
| `--environment` | Only sync specific environments (can be repeated) |

Examples:

```bash
# Sync all environments
openvox-code sync

# Sync only production and staging
openvox-code sync --environment production --environment staging

# Sync with cleanup of stale environments
openvox-code sync --clean
```

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
| `--environment` | Only deploy specific environments (can be repeated) |

Examples:

```bash
# Deploy from cache
openvox-code deploy

# Deploy using a lockfile
openvox-code deploy --lockfile openvox-code.lock

# Deploy only production
openvox-code deploy --environment production
```

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

Build an OCI container image containing the deployed environments. The image reference is specified using the `-t` flag (podman/docker style) or read from the `oci` section in the configuration file.

```bash
openvox-code build -t ghcr.io/example/puppet-envs:v1.0.0
```

| Flag | Description |
|---|---|
| `-t`, `--tag` | Image reference (`registry/image:tag`) |
| `--push` | Push image to registry after building |

When no `-t` flag is provided, the registry and tag are read from the `oci.registry` and `oci.tag` fields in the configuration file.

If `--push` is not specified, the image is saved locally as `openvox-code.tar`.

Examples:

```bash
# Build and save locally
openvox-code build -t ghcr.io/example/puppet-envs:v1.0.0

# Build and push in one step
openvox-code build -t ghcr.io/example/puppet-envs:v1.0.0 --push

# Build using registry from config
openvox-code build
```

### `openvox-code push`

Push a previously built OCI image to a container registry. The image reference can be passed as a positional argument or read from the configuration file.

```bash
openvox-code push ghcr.io/example/puppet-envs:v1.0.0
```

Examples:

```bash
# Push with explicit reference
openvox-code push ghcr.io/example/puppet-envs:v1.0.0

# Push using registry from config
openvox-code push
```

### `openvox-code lock`

Generate or update the lockfile by resolving all refs to concrete Git SHAs.

```bash
openvox-code lock --config openvox-code.yaml
```

Writes `openvox-code.lock` (or the path specified by `--lockfile`). The lockfile records the exact SHA for every module in every environment, enabling reproducible deploys.

| Flag | Description |
|---|---|
| `--lockfile` | Output path for lockfile |

### `openvox-code version`

Print version information including the build commit, date, Go version, and OS/architecture.

```bash
openvox-code version
```
