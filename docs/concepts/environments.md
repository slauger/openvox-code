# Environment Management

openvox-code supports two modes for managing Puppet environments: static (explicitly declared) and dynamic (branch-discovered). Both can be used together.

## Dynamic Environments

Dynamic environments are discovered automatically from Git branches in your control repository. Each branch becomes a Puppet environment.

### Branch Selection

Branch discovery is controlled by the `branchSelector` field on each source. It supports glob patterns for flexible matching.

**All branches** (empty `matchPatterns` matches everything):

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

**Specific branches** (exact names, no discovery needed):

```yaml
sources:
  - url: https://github.com/example/control-repo.git
    branchSelector:
      matchPatterns:
        - production
        - staging
        - development
```

**Glob patterns** (branches are discovered and filtered):

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
        - "feature_test_*"
```

The `matchPatterns` field accepts Go's `filepath.Match` glob syntax:

| Pattern | Matches |
|---|---|
| `*` | Any sequence of non-separator characters |
| `?` | Any single non-separator character |
| `[abc]` | Character class |
| `production` | Exact match |

When `matchPatterns` is empty, all branches are included. The `excludePatterns` field is always applied after matching -- a branch matching any exclude pattern is skipped even if it also matches an include pattern.

Branch names are sanitized for use as directory names (e.g., `feature/login` becomes `feature_login`).

### Module Files in Dynamic Environments

Each branch can include YAML module files that list the modules for that environment. Module files are specified as an array of `{name, required}` objects, where `name` supports glob patterns:

```yaml
sources:
  - url: https://github.com/example/control-repo.git
    branchSelector: {}
    modulefiles:
      - name: modules.yaml
        required: true
      - name: "modules.d/*.yaml"
        required: false
```

The module files live inside the control repo and are read per-branch after the control repo is checked out. They support both a Kubernetes-style envelope and a flat format:

**Kubernetes-style module file:**

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

**Flat module file:**

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

### Source-Level Modules

Sources can define modules and module sets that apply to all discovered branches. This avoids duplicating common module definitions in every branch's module file:

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
```

Source-level `modulesets` and `modules` are combined with per-branch module file modules. Per-branch module file definitions take precedence on name conflicts.

### Follow Branch

The `follow_branch` field on a module tells openvox-code to first try the branch matching the environment name. If that branch does not exist in the module repository, it falls back to the `ref` field (or HEAD if `ref` is empty):

```yaml
modules:
  - name: myapp
    git: https://github.com/example/puppet-myapp.git
    follow_branch: true
    ref: main              # fallback if the environment branch doesn't exist
```

This is useful for modules where developers work on feature branches that match the control repo branch names.

## Static Environments

Static environments are explicitly declared in the configuration file. They give you full control over the ref and module list for each environment.

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments

  environments:
    production:
      ref: v1.5.0
      modulesets:
        - common
      modules:
        - name: myapp
          git: https://github.com/example/puppet-myapp.git
          ref: v2.1.0

    staging:
      ref: staging
      modulesets:
        - common
      modules:
        - name: myapp
          git: https://github.com/example/puppet-myapp.git
          ref: main
```

Static environments are useful when you want to:

- Pin an environment to a specific tag or SHA rather than a branch tip
- Use different module versions per environment
- Mix module sets with per-environment module overrides

## Module Sets

Module sets let you define reusable groups of modules that can be shared across environments and sources:

```yaml
modulesets:
  common:
    - name: stdlib
      git: https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: v9.0.0
    - name: concat
      git: https://github.com/puppetlabs/puppetlabs-concat.git
      ref: v9.0.0

  monitoring:
    - name: prometheus
      git: https://github.com/example/puppet-prometheus.git
      ref: v1.2.0
    - name: grafana
      git: https://github.com/example/puppet-grafana.git
      ref: v3.0.0

environments:
  production:
    ref: production
    modulesets:
      - common
      - monitoring
```

When an environment references multiple module sets, all modules from all referenced sets are included. If a module appears in multiple sets, the last one wins.

## Custom Install Paths

Modules support `target_dir` and `install_as` fields for controlling where they are installed:

```yaml
modules:
  - name: role
    git: https://github.com/example/puppet-role.git
    ref: main
    target_dir: site        # install under site/ instead of modules/
  - name: puppetlabs-stdlib
    git: https://github.com/puppetlabs/puppetlabs-stdlib.git
    ref: v9.0.0
    install_as: stdlib      # directory name: modules/stdlib/
```

The default `target_dir` is `modules` and the default `install_as` is the module `name`.

## Overrides

Per-environment modules take precedence over module sets. This lets you pin a specific version of a module in one environment while using the default version everywhere else:

```yaml
modulesets:
  common:
    - name: stdlib
      git: https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: v9.0.0

environments:
  production:
    ref: production
    modulesets:
      - common
    modules:
      - name: stdlib
        git: https://github.com/puppetlabs/puppetlabs-stdlib.git
        ref: v8.6.0  # pinned to older version in production
```

## Precedence

When both dynamic and static environments exist with the same name, the static declaration wins. This lets you use branch discovery as a baseline while explicitly controlling specific environments.

## Lockfile

openvox-code supports a lockfile (`openvox-code.lock`) that records the exact Git SHA for every module in every environment. This ensures reproducible deploys.

```bash
# Generate or update the lockfile
openvox-code lock --config openvox-code.yaml

# Deploy using locked SHAs
openvox-code deploy --lockfile openvox-code.lock
```

The lockfile is a YAML file with the following structure:

```yaml
version: 1
generated_at: "2026-01-15T10:30:00Z"
environments:
  production:
    ref: v1.5.0
    control_repo_url: https://github.com/example/control-repo.git
    control_repo_sha: a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
    modules:
      - name: stdlib
        git: https://github.com/puppetlabs/puppetlabs-stdlib.git
        ref: v9.0.0
        sha: f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5
      - name: apache
        git: https://github.com/puppetlabs/puppetlabs-apache.git
        ref: v12.0.0
        sha: 1a2b3c4d5e6f1a2b3c4d5e6f1a2b3c4d5e6f1a2b
```

This is particularly useful for:

- Reproducible production deploys
- Auditing exactly which code is deployed
- Rolling back to a known-good state
