# Configuration Reference

Complete reference for the `openvox-code.yaml` configuration file.

## Configuration Format

openvox-code supports a Kubernetes-style YAML envelope with `apiVersion`, `kind`, and `spec` fields. A flat format (without the envelope) is also supported for backwards compatibility.

### Kubernetes-Style (Recommended)

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments
  sources:
    - url: https://github.com/example/control-repo.git
      branchSelector:
        matchPatterns: []
```

### Flat Format (Legacy)

```yaml
cachedir: /var/cache/openvox-code
environmentdir: /etc/puppetlabs/code/environments
sources:
  - url: https://github.com/example/control-repo.git
    branchSelector:
      matchPatterns: []
```

## Schema

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  # Optional: Include additional configuration files
  includes:
    - string               # File path or glob pattern

  # Required: Directory for bare clone caches
  cachedir: string

  # Required: Target directory for deployed environments
  environmentdir: string

  # Optional: Control repository sources for branch discovery
  sources:
    - url: string          # Git URL (required)
      branchSelector:      # Branch selection (optional)
        matchPatterns:      # Glob patterns to include (empty = all)
          - string
        excludePatterns:    # Glob patterns to exclude
          - string
      modulesets:           # Global module set names for all branches (optional)
        - string
      modules:              # Additional modules for all branches (optional)
        - name: string
          git: string
          ref: string
          follow_branch: bool
          target_dir: string
          install_as: string
      modulefiles:          # Per-branch module files from control repo (optional)
        - name: string      # File path or glob pattern within the repo
          required: bool    # Fail if file is missing

  # Optional: Reusable named groups of modules
  modulesets:
    <name>:
      - name: string       # Module name (required)
        git: string        # Git URL (required)
        ref: string        # Git ref (optional, empty = HEAD)
        follow_branch: bool   # Try environment branch first (optional)
        target_dir: string    # Parent directory (optional, default: "modules")
        install_as: string    # Directory name (optional, default: name)

  # Optional: Explicitly declared environments
  environments:
    <name>:
      ref: string          # Git ref for control repo (required)
      modulesets: list     # Module set names to include (optional)
      modules:             # Additional modules (optional)
        - name: string
          git: string
          ref: string
          follow_branch: bool
          target_dir: string
          install_as: string

  # Optional: Global overrides
  overrides:
    gitmirror: string      # Base URL for Git mirror (all URLs)
    gitmirrors:             # Per-host Git mirrors
      <host>: string       # host → mirror base URL

  # Optional: Git authentication
  git:
    ssh_key: string              # Default SSH key path
    ssh_known_hosts: string      # Default known_hosts file
    credential_helper: string    # Default Git credential helper
    credentials:                 # Per-host credentials
      <host>:
        ssh_key: string
        ssh_known_hosts: string
        credential_helper: string

  # Optional: Skip network operations
  offline: bool            # Default: false

  # Optional: OCI image configuration
  oci:
    registry: string       # OCI registry URL
    tag: string            # Image tag
    auth_config: string    # Path to Docker config.json for registry auth
```

## Field Reference

### `includes`

| | |
|---|---|
| Type | `list[string]` |
| Required | No |
| Default | `[]` |

List of file paths or glob patterns pointing to additional YAML configuration files. Included files are processed in order and deep-merged into the main configuration.

**Merge behavior:**

- Maps are merged recursively (e.g., `modulesets` from multiple files are combined).
- Lists are concatenated.
- Scalar values from later files override earlier ones.
- The main `openvox-code.yaml` serves as the base; included files are merged on top in the order specified.

Paths are relative to the directory containing the main configuration file. Glob patterns follow standard shell glob syntax.

```yaml
includes:
  - modulesets/*.yaml
  - environments/production.yaml
  - /etc/openvox-code/overrides.yaml
```

Included files can contain any valid top-level configuration fields (`cachedir`, `modulesets`, `environments`, etc.) but **cannot** contain `includes` themselves (no recursive includes).

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

#### `sources[].branchSelector`

| | |
|---|---|
| Type | `object` |
| Required | No |

Controls which branches are discovered as environments. Uses glob patterns for flexible matching.

##### `sources[].branchSelector.matchPatterns`

| | |
|---|---|
| Type | `list[string]` |
| Required | No |
| Default | `[]` (matches all branches) |

Glob patterns to include. When empty, all branches are matched. When patterns contain glob metacharacters (`*`, `?`, `[`), branch discovery is performed and branches are filtered against the patterns. When patterns are exact branch names (no globs), those specific branches are used directly without discovery.

##### `sources[].branchSelector.excludePatterns`

| | |
|---|---|
| Type | `list[string]` |
| Required | No |
| Default | `[]` |

Glob patterns to exclude. Applied after `matchPatterns`. A branch matching any exclude pattern is skipped even if it matches an include pattern.

#### `sources[].modulesets`

| | |
|---|---|
| Type | `list[string]` |
| Required | No |
| Default | `[]` |

Names of module sets to apply to all branches discovered from this source. The referenced module sets must be defined in the top-level `modulesets` map.

#### `sources[].modules`

| | |
|---|---|
| Type | `list[module]` |
| Required | No |
| Default | `[]` |

Additional modules applied to all branches discovered from this source.

#### `sources[].modulefiles`

| | |
|---|---|
| Type | `list[modulefileref]` |
| Required | No |
| Default | `[]` |

List of per-branch module files to read from inside the control repository. Each entry specifies a file path (or glob pattern) within the repo and whether the file is required.

##### `sources[].modulefiles[].name`

| | |
|---|---|
| Type | `string` |
| Required | Yes |

Path or glob pattern within the control repo pointing to a module file (e.g., `modules.yaml`, `modules/*.yaml`).

##### `sources[].modulefiles[].required`

| | |
|---|---|
| Type | `bool` |
| Required | No |
| Default | `false` |

When `true`, the sync fails if this module file is not found in a branch. When `false`, a missing file is silently skipped.

### `modulesets`

| | |
|---|---|
| Type | `map[string, list[module]]` |
| Required | No |
| Default | `{}` |

Named groups of modules. Keys are set names, values are lists of module definitions. Referenced by environments and sources via their `modulesets` fields.

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

Used in `modulesets`, `environments.<name>.modules`, and `sources[].modules`.

#### `name`

| | |
|---|---|
| Type | `string` |
| Required | Yes |

Module name. Used as the directory name under the target directory (default `modules/`) in the environment.

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
| Required | No |
| Default | HEAD |

Git ref to check out. Can be a branch name, tag, or full SHA. When empty, defaults to HEAD.

#### `follow_branch`

| | |
|---|---|
| Type | `bool` |
| Required | No |
| Default | `false` |

When `true`, openvox-code first tries to check out the branch matching the environment name. If that branch does not exist in the module repository, it falls back to `ref` (or HEAD if `ref` is empty).

#### `target_dir`

| | |
|---|---|
| Type | `string` |
| Required | No |
| Default | `modules` |

Parent directory for the module installation, relative to the environment root. Change this to install a module outside the default `modules/` directory (e.g., `site` for role/profile modules).

#### `install_as`

| | |
|---|---|
| Type | `string` |
| Required | No |
| Default | value of `name` |

Directory name to use when installing the module. Use this when the repository name does not match the desired directory name (e.g., install `puppetlabs-stdlib` as `stdlib`).

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

#### `overrides.gitmirrors`

| | |
|---|---|
| Type | `map[string, string]` |
| Required | No |

Per-host Git mirrors. Keys are hostnames, values are mirror base URLs. When a Git URL matches a host key, it is rewritten to use the corresponding mirror. Per-host mirrors take precedence over `gitmirror`.

```yaml
overrides:
  gitmirrors:
    github.com: https://github-mirror.internal.example.com
    gitlab.com: https://gitlab-mirror.internal.example.com
```

### `git`

| | |
|---|---|
| Type | `object` |
| Required | No |
| Default | `{}` |

Git authentication settings. Supports both default credentials and per-host credential resolution.

#### `git.ssh_key`

| | |
|---|---|
| Type | `string` |
| Required | No |

Default path to the SSH private key used for Git operations.

#### `git.ssh_known_hosts`

| | |
|---|---|
| Type | `string` |
| Required | No |

Default path to the SSH known_hosts file.

#### `git.credential_helper`

| | |
|---|---|
| Type | `string` |
| Required | No |

Default Git credential helper command.

#### `git.credentials`

| | |
|---|---|
| Type | `map[string, credential]` |
| Required | No |

Per-host credential overrides. Keys are hostnames. When a Git URL matches a host, the host-specific credentials are used instead of the defaults.

```yaml
git:
  ssh_key: /root/.ssh/id_rsa
  ssh_known_hosts: /root/.ssh/known_hosts
  credentials:
    gitlab.example.com:
      ssh_key: /root/.ssh/gitlab_key
      ssh_known_hosts: /root/.ssh/known_hosts_gitlab
    github.com:
      credential_helper: "!gh auth git-credential"
```

### `offline`

| | |
|---|---|
| Type | `bool` |
| Required | No |
| Default | `false` |

When `true`, all network operations are skipped. Commands that require network access (like `mirror` and `sync`) will fail. The `deploy` command works normally from the local cache.

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

#### `oci.auth_config`

| | |
|---|---|
| Type | `string` |
| Required | No |

Path to a Docker `config.json` file for registry authentication.

## Module File Format

Module files are YAML files read from inside a control repository branch. They list the modules to deploy for that branch. Module files support both a Kubernetes-style envelope and a flat format.

### Kubernetes-Style Module File

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: ModuleFile
spec:
  modules:
    - name: stdlib
      git: https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: v9.0.0
    - name: apache
      git: https://github.com/puppetlabs/puppetlabs-apache.git
      ref: v12.0.0
  exclude:
    - obsolete_module
```

### Flat Module File

```yaml
modules:
  - name: stdlib
    git: https://github.com/puppetlabs/puppetlabs-stdlib.git
    ref: v9.0.0
  - name: apache
    git: https://github.com/puppetlabs/puppetlabs-apache.git
    ref: v12.0.0
exclude:
  - obsolete_module
```

The `exclude` field allows a branch to remove modules that would otherwise be inherited from source-level `modulesets` or `modules`.

## Example Configurations

### Minimal: Branch Discovery

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments

  sources:
    - url: https://github.com/example/control-repo.git
      branchSelector: {}
```

### Branch Discovery with Patterns

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments

  sources:
    - url: https://github.com/example/control-repo.git
      branchSelector:
        matchPatterns:
          - production
          - staging
          - "feature_*"
        excludePatterns:
          - "feature_wip_*"
      modulefiles:
        - name: modules.yaml
          required: true
        - name: "modules.d/*.yaml"
          required: false
```

### Production: Pinned Environments with Module Sets

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
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

### Source-Level Modules and Module Sets

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments

  modulesets:
    base:
      - name: stdlib
        git: https://github.com/puppetlabs/puppetlabs-stdlib.git
        ref: v9.0.0

  sources:
    - url: https://github.com/example/control-repo.git
      branchSelector: {}
      modulesets:
        - base
      modules:
        - name: myapp
          git: https://github.com/example/puppet-myapp.git
          follow_branch: true
      modulefiles:
        - name: modules.yaml
          required: false
```

### Follow Branch and Custom Install Paths

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments

  modulesets:
    base:
      - name: stdlib
        git: https://github.com/puppetlabs/puppetlabs-stdlib.git
        ref: v9.0.0
      - name: role
        git: https://github.com/example/puppet-role.git
        follow_branch: true
        target_dir: site
      - name: profile
        git: https://github.com/example/puppet-profile.git
        follow_branch: true
        target_dir: site

  environments:
    production:
      ref: production
      modulesets:
        - base
```

### Per-Host Git Credentials

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments

  git:
    ssh_key: /root/.ssh/id_rsa
    ssh_known_hosts: /root/.ssh/known_hosts
    credentials:
      gitlab.example.com:
        ssh_key: /root/.ssh/gitlab_deploy_key
        ssh_known_hosts: /root/.ssh/known_hosts_gitlab
      github.com:
        credential_helper: "!gh auth git-credential"

  sources:
    - url: git@github.com:example/control-repo.git
      branchSelector: {}
```

### Air-Gapped: Offline with Mirrors

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /mnt/puppet-cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments
  offline: true

  overrides:
    gitmirrors:
      github.com: https://github-mirror.internal.example.com
      gitlab.com: https://gitlab-mirror.internal.example.com

  sources:
    - url: https://github.com/example/control-repo.git
      branchSelector:
        matchPatterns:
          - production
          - staging
```

### Kubernetes: OCI Output

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /tmp/openvox-code-cache
  environmentdir: /tmp/openvox-code-envs

  sources:
    - url: https://github.com/example/control-repo.git
      branchSelector:
        matchPatterns:
          - production

  oci:
    registry: ghcr.io/example/puppet-environments
    tag: latest
    auth_config: /root/.docker/config.json
```

### Split Configuration with Includes

Main file `openvox-code.yaml`:

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments

  includes:
    - modulesets/*.yaml
    - environments/*.yaml
```

`modulesets/base.yaml`:

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  modulesets:
    base:
      - name: stdlib
        git: https://github.com/puppetlabs/puppetlabs-stdlib.git
        ref: v9.0.0
      - name: concat
        git: https://github.com/puppetlabs/puppetlabs-concat.git
        ref: v9.0.0
```

`environments/production.yaml`:

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  environments:
    production:
      ref: v1.5.0
      modulesets:
        - base
```
