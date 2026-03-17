# Configuration Format

openvox-code uses a single YAML file (`openvox-code.yaml`) to define all aspects of environment deployment. This replaces the Ruby-based Puppetfile format used by r10k and g10k.

## Minimal Example

```yaml
cachedir: /var/cache/openvox-code
environmentdir: /etc/puppetlabs/code/environments

sources:
  - url: https://github.com/example/control-repo.git
    branches: all
```

This fetches all branches from the control repo and deploys each branch as an environment.

## Full Example

```yaml
cachedir: /var/cache/openvox-code
environmentdir: /etc/puppetlabs/code/environments

sources:
  - url: https://github.com/example/control-repo.git
    branches: all
    modulefile: modules.yaml

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
      ref: main

environments:
  production:
    ref: production
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
  gitmirror: https://git-mirror.internal.example.com

offline: false

oci:
  registry: ghcr.io/example/puppet-environments
  tag: latest
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

List of control repositories. Each source can discover environments from branches.

```yaml
sources:
  - url: https://github.com/example/control-repo.git
    branches: all
    modulefile: modules.yaml
```

| Field | Type | Description |
|---|---|---|
| `url` | string | Git URL of the control repository |
| `branches` | string or list | `all`, a single branch name, or a list of branch names |
| `modulefile` | string | Path within the repo to a YAML file listing modules (optional) |

### `modulesets`

Named groups of modules that can be referenced by multiple environments. This avoids duplication when several environments share the same module set.

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
    ref: production
    modulesets:
      - common
    modules:
      - name: myapp
        git: https://github.com/example/puppet-myapp.git
        ref: v2.1.0
```

| Field | Type | Description |
|---|---|---|
| `ref` | string | Git ref (branch, tag, or SHA) for the control repo |
| `modulesets` | list | Named module sets to include |
| `modules` | list | Additional modules specific to this environment |

### `modules`

Each module entry specifies a Git repository and ref:

```yaml
modules:
  - name: apache
    git: https://github.com/puppetlabs/puppetlabs-apache.git
    ref: v12.0.0
```

| Field | Type | Description |
|---|---|---|
| `name` | string | Module name (used as the directory name under `modules/`) |
| `git` | string | Git URL of the module repository |
| `ref` | string | Git ref to check out (branch, tag, or SHA) |

### `overrides`

Global overrides that affect all Git operations.

```yaml
overrides:
  gitmirror: https://git-mirror.internal.example.com
```

| Field | Type | Description |
|---|---|---|
| `gitmirror` | string | Base URL of a Git mirror; rewrites all Git URLs to use this host |

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
```

| Field | Type | Description |
|---|---|---|
| `registry` | string | OCI registry URL to push images to |
| `tag` | string | Image tag |

## Branch Discovery with Module Files

When using `sources` with `branches: all`, openvox-code discovers branches and deploys each as an environment. You can include a `modulefile` path that points to a YAML file within the control repo that lists modules for that branch:

```yaml
# modules.yaml (inside the control repo)
modules:
  - name: stdlib
    git: https://github.com/puppetlabs/puppetlabs-stdlib.git
    ref: v9.0.0
  - name: apache
    git: https://github.com/puppetlabs/puppetlabs-apache.git
    ref: v12.0.0
```

## Static vs Dynamic Environments

You can use `sources` (dynamic, branch-discovered) and `environments` (static, explicitly declared) together. Explicitly declared environments take precedence when there is a naming conflict.

See [Environment Management](environments.md) for more details.
