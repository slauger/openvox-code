# Configuration Format

openvox-code uses a single YAML file (`openvox-code.yaml`) to define all aspects of environment deployment. This replaces the Ruby-based Puppetfile format used by r10k and g10k.

The configuration supports a Kubernetes-style envelope with `apiVersion`, `kind`, and `spec` fields. A flat format (without the envelope) is also accepted for backwards compatibility, but the Kubernetes-style format is recommended for new configurations.

## Minimal Example

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

This fetches all branches from the control repo and deploys each branch as an environment.

## Full Example

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
      modulesets:
        - common
      modules:
        - name: myapp
          git: https://github.com/example/puppet-myapp.git
          follow_branch: true
      modulefiles:
        - name: modules.yaml
          required: true
        - name: "modules.d/*.yaml"
          required: false

  modulesets:
    common:
      - name: stdlib
        git: https://github.com/puppetlabs/puppetlabs-stdlib.git
        ref: v9.0.0
      - name: concat
        git: https://github.com/puppetlabs/puppetlabs-concat.git
        ref: v9.0.0

    webservers:
      - name: apache
        git: https://github.com/puppetlabs/puppetlabs-apache.git
        ref: v12.0.0
      - name: nginx
        git: https://github.com/example/puppet-nginx.git
        follow_branch: true

  environments:
    production:
      ref: v1.5.0
      modulesets:
        - common
        - webservers
      modules:
        - name: myapp
          git: https://github.com/example/puppet-myapp.git
          ref: v2.1.0

    staging:
      ref: staging
      modulesets:
        - common
        - webservers
      modules:
        - name: myapp
          git: https://github.com/example/puppet-myapp.git
          ref: main

  overrides:
    gitmirrors:
      github.com: https://github-mirror.internal.example.com

  git:
    ssh_key: /root/.ssh/id_rsa
    ssh_known_hosts: /root/.ssh/known_hosts
    credentials:
      gitlab.example.com:
        ssh_key: /root/.ssh/gitlab_deploy_key
        ssh_known_hosts: /root/.ssh/known_hosts_gitlab

  offline: false

  oci:
    registry: ghcr.io/example/puppet-environments
    tag: latest
    auth_config: /root/.docker/config.json
```

## Top-Level Fields

### `cachedir`

Directory for bare clone caches. Each Git repository is cloned once here and shared across all environments.

```yaml
cachedir: /var/cache/openvox-code
```

### `environmentdir`

Target directory where environments are deployed.

```yaml
environmentdir: /etc/puppetlabs/code/environments
```

### `sources`

List of control repositories. Each source discovers environments from branches and can define modules that apply to all discovered branches.

```yaml
sources:
  - url: https://github.com/example/control-repo.git
    branchSelector:
      matchPatterns:
        - production
        - staging
        - "feature_*"
      excludePatterns:
        - "feature_wip_*"
    modulesets:
      - common
    modules:
      - name: myapp
        git: https://github.com/example/puppet-myapp.git
        follow_branch: true
    modulefiles:
      - name: modules.yaml
        required: true
```

| Field | Type | Description |
|---|---|---|
| `url` | string | Git URL of the control repository |
| `branchSelector` | object | Branch selection with `matchPatterns` and `excludePatterns` (glob patterns) |
| `modulesets` | list | Names of module sets to apply to all discovered branches |
| `modules` | list | Additional modules for all discovered branches |
| `modulefiles` | list | Per-branch module files to read from the control repo (`{name, required}`) |

### `modulesets`

Named groups of modules that can be referenced by environments and sources. This avoids duplication when several environments share the same module set.

```yaml
modulesets:
  common:
    - name: stdlib
      git: https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: v9.0.0
```

### `environments`

Explicitly declared environments with pinned refs and module lists.

```yaml
environments:
  production:
    ref: v1.5.0
    modulesets:
      - common
    modules:
      - name: myapp
        git: https://github.com/example/puppet-myapp.git
        ref: v2.1.0
```

| Field | Type | Description |
|---|---|---|
| `ref` | string | Git ref (branch, tag, or SHA) for the control repo (required) |
| `modulesets` | list | Named module sets to include |
| `modules` | list | Additional modules specific to this environment |

### Module Definition

Each module entry specifies a Git repository and optional ref:

```yaml
modules:
  - name: apache
    git: https://github.com/puppetlabs/puppetlabs-apache.git
    ref: v12.0.0
  - name: role
    git: https://github.com/example/puppet-role.git
    follow_branch: true
    target_dir: site
```

| Field | Type | Description |
|---|---|---|
| `name` | string | Module name (used as directory name, or see `install_as`) |
| `git` | string | Git URL of the module repository |
| `ref` | string | Git ref to check out (optional, empty = HEAD) |
| `follow_branch` | bool | Try environment branch first, fall back to `ref`/HEAD |
| `target_dir` | string | Parent directory (default: `modules`) |
| `install_as` | string | Directory name (default: value of `name`) |

### `overrides`

Global overrides that affect all Git operations.

```yaml
overrides:
  gitmirror: https://git-mirror.internal.example.com
  gitmirrors:
    github.com: https://github-mirror.internal.example.com
    gitlab.com: https://gitlab-mirror.internal.example.com
```

| Field | Type | Description |
|---|---|---|
| `gitmirror` | string | Base URL of a Git mirror; rewrites all Git URLs to use this host |
| `gitmirrors` | map | Per-host Git mirrors (host -> mirror base URL); takes precedence over `gitmirror` |

### `git`

Git authentication settings with per-host credential resolution.

```yaml
git:
  ssh_key: /root/.ssh/id_rsa
  ssh_known_hosts: /root/.ssh/known_hosts
  credentials:
    gitlab.example.com:
      ssh_key: /root/.ssh/gitlab_deploy_key
    github.com:
      credential_helper: "!gh auth git-credential"
```

| Field | Type | Description |
|---|---|---|
| `ssh_key` | string | Default SSH private key path |
| `ssh_known_hosts` | string | Default SSH known_hosts file |
| `credential_helper` | string | Default Git credential helper |
| `credentials` | map | Per-host credential overrides (`host -> {ssh_key, ssh_known_hosts, credential_helper}`) |

When a Git URL is accessed, openvox-code checks if the URL's host matches any key in `credentials`. If a match is found, the host-specific credentials are used. Otherwise, the default `ssh_key`/`credential_helper` values apply.

### `offline`

When set to `true`, skips all network operations and deploys only from the local cache. Useful for air-gapped environments.

```yaml
offline: true
```

### `oci`

Configuration for building OCI container images.

```yaml
oci:
  registry: ghcr.io/example/puppet-environments
  tag: latest
  auth_config: /root/.docker/config.json
```

| Field | Type | Description |
|---|---|---|
| `registry` | string | OCI registry URL to push images to |
| `tag` | string | Image tag (default: `latest`) |
| `auth_config` | string | Path to Docker config.json for registry authentication |

## Branch Discovery with Module Files

When using `sources` with branch discovery, openvox-code discovers branches and deploys each as an environment. You can include `modulefiles` that point to YAML files within the control repo:

```yaml
sources:
  - url: https://github.com/example/control-repo.git
    branchSelector: {}
    modulefiles:
      - name: modules.yaml
        required: true
```

The module file supports both Kubernetes-style and flat formats. See the [Configuration Reference](../reference/configuration.md#module-file-format) for the full format.

## Splitting Configuration with Includes

As configurations grow, a single `openvox-code.yaml` can become hard to manage. The `includes` field lets you split your configuration across multiple files:

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

This is useful for:

- **Team ownership**: Each team maintains their own moduleset or environment file.
- **Reusability**: Share common modulesets across projects by including a shared file.
- **Separation of concerns**: Keep base config, module definitions, and environment declarations in separate files.

### Merge Behavior

Included files are deep-merged into the main configuration in order:

- **Maps** (like `modulesets`, `environments`) are merged recursively. Keys from different files are combined; on conflict, later files win.
- **Lists** (like `sources`) are concatenated.
- **Scalars** (like `cachedir`, `offline`) from later files override earlier values.

Glob patterns are expanded in alphabetical order. Paths are relative to the main config file's directory. Recursive includes (includes within included files) are not supported.

See the [Configuration Reference](../reference/configuration.md#includes) for the full field specification.

## Static vs Dynamic Environments

You can use `sources` (dynamic, branch-discovered) and `environments` (static, explicitly declared) together. Explicitly declared environments take precedence when there is a naming conflict.

See [Environment Management](environments.md) for more details.
