# Configuration Reference

Complete reference for the `openvox-code.yaml` configuration file.

## Schema

```yaml
# Required: Directory for bare clone caches
cachedir: string

# Required: Target directory for deployed environments
environmentdir: string

# Optional: Control repository sources for branch discovery
sources:
  - url: string          # Git URL (required)
    branches: string|list # "all" or list of branch names (required)
    modulefile: string    # Path to module list within repo (optional)

# Optional: Reusable named groups of modules
modulesets:
  <name>:
    - name: string       # Module name (required)
      git: string        # Git URL (required)
      ref: string        # Git ref (required)

# Optional: Explicitly declared environments
environments:
  <name>:
    ref: string          # Git ref for control repo (required)
    modulesets: list     # Module set names to include (optional)
    modules:             # Additional modules (optional)
      - name: string     # Module name (required)
        git: string      # Git URL (required)
        ref: string      # Git ref (required)

# Optional: Global overrides
overrides:
  gitmirror: string      # Base URL for Git mirror

# Optional: Skip network operations
offline: bool            # Default: false

# Optional: OCI image configuration
oci:
  registry: string       # OCI registry URL
  tag: string            # Image tag
```

## Field Reference

### `cachedir`

| | |
|---|---|
| Type | `string` |
| Required | Yes |
| Default | — |

Absolute path to the directory where bare Git clones are stored. Created automatically if it does not exist.

```yaml
cachedir: /var/cache/openvox-code
```

### `environmentdir`

| | |
|---|---|
| Type | `string` |
| Required | Yes |
| Default | — |

Absolute path to the directory where Puppet environments are deployed. Each environment becomes a subdirectory.

```yaml
environmentdir: /etc/puppetlabs/code/environments
```

### `sources`

| | |
|---|---|
| Type | `list[source]` |
| Required | No |
| Default | `[]` |

List of control repository sources for branch-based environment discovery.

#### `sources[].url`

| | |
|---|---|
| Type | `string` |
| Required | Yes |

Git URL of the control repository. Supports HTTPS and SSH.

#### `sources[].branches`

| | |
|---|---|
| Type | `string` or `list[string]` |
| Required | Yes |

Which branches to discover as environments. Use `"all"` for all branches, or provide a list of specific branch names.

#### `sources[].modulefile`

| | |
|---|---|
| Type | `string` |
| Required | No |
| Default | — |

Path relative to the repository root of a YAML file listing modules. Read per-branch during discovery.

### `modulesets`

| | |
|---|---|
| Type | `map[string, list[module]]` |
| Required | No |
| Default | `{}` |

Named groups of modules. Keys are set names, values are lists of module definitions. Referenced by environments via the `modulesets` field.

### `environments`

| | |
|---|---|
| Type | `map[string, environment]` |
| Required | No |
| Default | `{}` |

Explicitly declared environments. Keys are environment names (used as directory names).

#### `environments.<name>.ref`

| | |
|---|---|
| Type | `string` |
| Required | Yes |

Git ref (branch, tag, or SHA) for the control repository checkout.

#### `environments.<name>.modulesets`

| | |
|---|---|
| Type | `list[string]` |
| Required | No |
| Default | `[]` |

List of module set names to include. Modules from all listed sets are merged.

#### `environments.<name>.modules`

| | |
|---|---|
| Type | `list[module]` |
| Required | No |
| Default | `[]` |

Additional modules specific to this environment. These take precedence over modules from module sets if there is a name conflict.

### Module Definition

Used in `modulesets` and `environments.<name>.modules`.

#### `name`

| | |
|---|---|
| Type | `string` |
| Required | Yes |

Module name. Used as the directory name under `modules/` in the environment.

#### `git`

| | |
|---|---|
| Type | `string` |
| Required | Yes |

Git URL of the module repository.

#### `ref`

| | |
|---|---|
| Type | `string` |
| Required | Yes |

Git ref to check out. Can be a branch name, tag, or full SHA.

### `overrides`

| | |
|---|---|
| Type | `object` |
| Required | No |
| Default | `{}` |

#### `overrides.gitmirror`

| | |
|---|---|
| Type | `string` |
| Required | No |

Base URL of a Git mirror. When set, all Git URLs are rewritten to use this host. The path component of the original URL is preserved.

For example, with `gitmirror: https://mirror.example.com`:

- `https://github.com/puppetlabs/puppetlabs-stdlib.git` becomes `https://mirror.example.com/puppetlabs/puppetlabs-stdlib.git`

### `offline`

| | |
|---|---|
| Type | `bool` |
| Required | No |
| Default | `false` |

When `true`, all network operations are skipped. Commands that require network access (like `mirror`) will fail. The `deploy` command works normally from the local cache.

### `oci`

| | |
|---|---|
| Type | `object` |
| Required | No |
| Default | — |

Configuration for OCI image output.

#### `oci.registry`

| | |
|---|---|
| Type | `string` |
| Required | Yes (if `oci` is set) |

OCI registry URL to push images to.

#### `oci.tag`

| | |
|---|---|
| Type | `string` |
| Required | No |
| Default | `latest` |

Image tag.

## Example Configurations

### Minimal: Branch Discovery

```yaml
cachedir: /var/cache/openvox-code
environmentdir: /etc/puppetlabs/code/environments

sources:
  - url: https://github.com/example/control-repo.git
    branches: all
```

### Production: Pinned Environments with Lockfile

```yaml
cachedir: /var/cache/openvox-code
environmentdir: /etc/puppetlabs/code/environments

modulesets:
  base:
    - name: stdlib
      git: https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: v9.0.0
    - name: concat
      git: https://github.com/puppetlabs/puppetlabs-concat.git
      ref: v9.0.0
    - name: firewall
      git: https://github.com/puppetlabs/puppetlabs-firewall.git
      ref: v8.0.0

environments:
  production:
    ref: v1.5.0
    modulesets:
      - base
  staging:
    ref: staging
    modulesets:
      - base
```

### Air-Gapped: Offline with Mirror

```yaml
cachedir: /mnt/puppet-cache/openvox-code
environmentdir: /etc/puppetlabs/code/environments
offline: true

overrides:
  gitmirror: https://git-mirror.internal.example.com

sources:
  - url: https://github.com/example/control-repo.git
    branches:
      - production
      - staging
```

### Kubernetes: OCI Output

```yaml
cachedir: /tmp/openvox-code-cache
environmentdir: /tmp/openvox-code-envs

sources:
  - url: https://github.com/example/control-repo.git
    branches:
      - production

oci:
  registry: ghcr.io/example/puppet-environments
  tag: latest
```
