# Migration Guide: r10k/g10k → openvox-code

This guide helps you migrate from r10k or g10k to openvox-code.

## Quick Comparison

| Feature | r10k | g10k | openvox-code |
|---------|------|------|-------------|
| Language | Ruby | Go | Go |
| Config format | Puppetfile (Ruby DSL) | Puppetfile (Ruby DSL) | YAML (K8s-style) |
| Branch discovery | `r10k.yaml` sources | `g10k.yaml` sources | `branchSelector` with glob patterns |
| Module source | Git, Forge | Git, Forge | Git |
| Parallel fetch | Limited | Yes | Yes (configurable `--parallel`) |
| OCI output | No | No | Yes (`build` / `push`) |
| Lockfile | No | No | Yes (`lock` command) |
| Offline mode | No | Partial | Yes (`offline: true`) |
| Retry on failure | No | No | Yes (exponential backoff) |
| Per-host credentials | No | No | Yes (`git.credentials`) |

## Step 1: Convert Your r10k Configuration

### r10k.yaml → openvox-code.yaml

**Before (r10k.yaml):**

```yaml
sources:
  puppet:
    remote: https://github.com/example/puppet-control.git
    basedir: /etc/puppetlabs/code/environments
```

**After (openvox-code.yaml):**

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments
  sources:
    - url: https://github.com/example/puppet-control.git
      branchSelector: {}
```

### Multiple Sources

**Before:**

```yaml
sources:
  puppet:
    remote: https://github.com/example/puppet-control.git
    basedir: /etc/puppetlabs/code/environments
  hiera:
    remote: https://github.com/example/hiera-data.git
    basedir: /etc/puppetlabs/code/hieradata
```

**After:**

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /var/cache/openvox-code
  environmentdir: /etc/puppetlabs/code/environments
  sources:
    - url: https://github.com/example/puppet-control.git
      branchSelector: {}
      modulefiles:
        - name: modules.yaml
          required: true
```

!!! note
    openvox-code uses a single `environmentdir`. Each branch from the control repo becomes a subdirectory. Hiera data should live inside the control repo (as `hieradata/` or `data/`), not as a separate source.

## Step 2: Convert Your Puppetfile

### Puppetfile → modules.yaml

**Before (Puppetfile):**

```ruby
mod 'puppetlabs-stdlib', '9.7.0'

mod 'apache',
  :git    => 'https://github.com/puppetlabs/puppetlabs-apache.git',
  :tag    => 'v12.2.0'

mod 'profiles',
  :git            => 'https://github.com/example/puppet-profiles.git',
  :default_branch => 'main'
```

**After (modules.yaml in control repo):**

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: ModuleFile
spec:
  modules:
    - name: stdlib
      git: https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: v9.7.0

    - name: apache
      git: https://github.com/puppetlabs/puppetlabs-apache.git
      ref: v12.2.0

    - name: profiles
      git: https://github.com/example/puppet-profiles.git
      follow_branch: true
      ref: main
```

### Puppetfile Field Mapping

| Puppetfile | modules.yaml | Notes |
|-----------|-------------|-------|
| `mod 'name', 'version'` | Not supported | Use Git source with tag ref |
| `:git => 'url'` | `git: url` | |
| `:tag => 'v1.0'` | `ref: v1.0` | |
| `:branch => 'main'` | `ref: main` | |
| `:commit => 'abc123'` | `ref: abc123` | |
| `:ref => 'v1.0'` | `ref: v1.0` | |
| `:default_branch => 'main'` | `follow_branch: true` + `ref: main` | Tries environment branch first |
| `:install_path => 'site'` | `target_dir: site` | |
| Forge modules | Not supported | Use Git URL of the module |

### Key Differences

**No Forge support:** openvox-code uses Git exclusively. For Forge modules, use their Git repository URL instead:

```yaml
# Instead of: mod 'puppetlabs-stdlib', '9.7.0'
# Use:
- name: stdlib
  git: https://github.com/puppetlabs/puppetlabs-stdlib.git
  ref: v9.7.0
```

**`follow_branch` replaces `:default_branch`:** The semantics are similar but clearer — `follow_branch: true` tries the environment branch name first, then falls back to `ref`.

**No ref means HEAD:** Omitting `ref` checks out the default branch of the repository (HEAD), unlike r10k where `:ref` is required.

## Step 3: Branch Filtering

### r10k: No filtering (all branches)

r10k deploys all branches by default. To exclude branches, you needed external scripting.

### openvox-code: branchSelector

```yaml
sources:
  - url: https://github.com/example/puppet-control.git
    branchSelector:
      matchPatterns:
        - "production"
        - "staging"
        - "feature/*"
      excludePatterns:
        - "feature/wip-*"
```

An empty `branchSelector: {}` matches all branches (same as r10k default).

## Step 4: Global Modules (Module Sets)

r10k defines modules per-environment in Puppetfiles. openvox-code adds global module sets that apply across environments:

```yaml
spec:
  modulesets:
    base:
      - name: stdlib
        git: https://github.com/puppetlabs/puppetlabs-stdlib.git
        ref: v9.7.0
      - name: concat
        git: https://github.com/puppetlabs/puppetlabs-concat.git
        ref: v9.1.0

  sources:
    - url: https://github.com/example/puppet-control.git
      branchSelector: {}
      modulesets: [base]
      modulefiles:
        - name: modules.yaml
          required: false
```

Per-branch `modules.yaml` can override or exclude global modules:

```yaml
# modules.yaml in a specific branch
exclude:
  - concat
modules:
  - name: stdlib
    git: https://github.com/puppetlabs/puppetlabs-stdlib.git
    ref: v10.0.0  # override version for this branch
```

## Step 5: Switch Commands

| r10k / g10k | openvox-code | Notes |
|------------|-------------|-------|
| `r10k deploy environment` | `openvox-code sync` | Full fetch + deploy |
| `r10k deploy environment -p` | `openvox-code sync` | Modules are always included |
| `r10k deploy environment production` | `openvox-code sync --environment production` | |
| `r10k deploy module stdlib` | Not yet supported | Deploy specific module |
| `r10k puppetfile install` | `openvox-code deploy` | Deploy from cache |
| (no equivalent) | `openvox-code mirror` | Fetch only, no deploy |
| (no equivalent) | `openvox-code build -t reg:tag` | Build OCI image |
| (no equivalent) | `openvox-code push reg:tag` | Push OCI image |
| (no equivalent) | `openvox-code lock` | Generate lockfile |
| (no equivalent) | `openvox-code diff` | Show pending changes |

## Step 6: OCI Workflow (New)

The biggest advantage of openvox-code over r10k/g10k: build once, deploy everywhere.

```bash
# In CI/CD pipeline:
openvox-code sync --config openvox-code.yaml
openvox-code build -t ghcr.io/example/puppet-envs:v1.0.0 --push
```

The OCI image is consumed by [openvox-operator](https://github.com/slauger/openvox-operator) as a Kubernetes Image Volume — no Git access needed on Puppet servers.

## Common Pitfalls

1. **Forge modules:** Replace `mod 'author-name', 'version'` with the Git URL. Most Puppet Forge modules have a GitHub mirror.

2. **Environment directory:** For OCI workflow, use a temp directory (`/tmp/openvox-code/environments`). For legacy workflow, keep `/etc/puppetlabs/code/environments`.

3. **SSH keys:** Configure in the `git` section, not via `~/.ssh/config`:
   ```yaml
   git:
     ssh_key: /path/to/deploy_key
   ```

4. **Branch naming:** Puppet doesn't allow hyphens in environment names. Branches like `feature/my-feature` will create environments with hyphens, which may cause issues. Consider using underscores.
