# Environment Management

openvox-code supports two modes for managing Puppet environments: static (explicitly declared) and dynamic (branch-discovered). Both can be used together.

## Dynamic Environments

Dynamic environments are discovered automatically from Git branches in your control repository. Each branch becomes a Puppet environment.

```yaml
sources:
  - url: https://github.com/example/control-repo.git
    branches: all
```

With `branches: all`, openvox-code lists all branches in the control repo and creates an environment for each one. Branch names are sanitized for use as directory names (e.g., `feature/login` becomes `feature_login`).

You can also limit discovery to specific branches:

```yaml
sources:
  - url: https://github.com/example/control-repo.git
    branches:
      - production
      - staging
      - development
```

### Module Files in Dynamic Environments

Each branch can include a YAML module file that lists the modules for that environment:

```yaml
sources:
  - url: https://github.com/example/control-repo.git
    branches: all
    modulefile: modules.yaml
```

The `modules.yaml` file lives inside the control repo and is read per-branch:

```yaml
# modules.yaml
modules:
  - name: stdlib
    git: https://github.com/puppetlabs/puppetlabs-stdlib.git
    ref: v9.0.0
  - name: apache
    git: https://github.com/puppetlabs/puppetlabs-apache.git
    ref: v12.0.0
```

## Static Environments

Static environments are explicitly declared in the configuration file. They give you full control over the ref and module list for each environment.

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

Module sets let you define reusable groups of modules that can be shared across environments:

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
openvox-code sync --config openvox-code.yaml --lockfile openvox-code.lock
```

The lockfile is a YAML file that maps each environment and module to a specific commit SHA:

```yaml
environments:
  production:
    control: a1b2c3d4e5f6
    modules:
      stdlib: f6e5d4c3b2a1
      apache: 1a2b3c4d5e6f
  staging:
    control: b2c3d4e5f6a1
    modules:
      stdlib: f6e5d4c3b2a1
      apache: 2b3c4d5e6f1a
```

This is particularly useful for:

- Reproducible production deploys
- Auditing exactly which code is deployed
- Rolling back to a known-good state
