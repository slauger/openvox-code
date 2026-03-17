# Module Caching

openvox-code uses a bare clone cache to minimize network traffic and enable offline deployments.

## Bare Clone Cache

Every Git repository referenced in the configuration is cloned exactly once as a bare repository in the cache directory. These bare clones are shared across all environments.

```
/var/cache/openvox-code/git/
├── github.com-puppetlabs-puppetlabs-stdlib.git/
├── github.com-puppetlabs-puppetlabs-apache.git/
├── github.com-puppetlabs-puppetlabs-concat.git/
└── github.com-example-control-repo.git/
```

Cache directory names are derived from the Git URL by replacing `/` and `:` with `-` to create a flat, filesystem-safe structure.

### How It Works

1. **First run**: `git clone --bare` for each unique repository URL
2. **Subsequent runs**: `git fetch` to update existing bare clones
3. **Deploy**: `git checkout` from the bare clone into the environment directory

Because the cache stores bare repositories, it contains all branches and tags — any ref can be checked out without additional network access.

## Content-Addressable by Git SHA

When a lockfile is present, openvox-code resolves every module to a specific Git SHA. Since Git content is inherently content-addressable, this means:

- Two environments using the same module at the same SHA share the same cached data
- Cache lookups are exact — a SHA either exists in the bare clone or it doesn't
- No ambiguity from branch names that may have moved since last fetch

## Offline Mode

The cache enables a clean separation between network operations and local operations.

### Mirror Phase (requires network)

```bash
openvox-code mirror --config openvox-code.yaml
```

This fetches all repositories into the bare clone cache. It is the only command that requires network access.

### Deploy Phase (offline)

```bash
openvox-code deploy --config openvox-code.yaml
```

This reads from the local cache and writes environments to disk. No network access is required.

### Full Offline Workflow

In air-gapped or restricted environments, you can:

1. Run `openvox-code mirror` on a machine with network access
2. Copy the cache directory to the target machine
3. Run `openvox-code deploy` on the target machine

Alternatively, set `offline: true` in the configuration to skip all network operations:

```yaml
offline: true
cachedir: /var/cache/openvox-code
environmentdir: /etc/puppetlabs/code/environments
```

## Cache Invalidation

The cache is invalidated naturally through Git operations:

- **Branch refs**: Updated on every `git fetch` during the mirror phase
- **Tags**: Fetched once and immutable (Git tags should not be force-pushed)
- **SHAs**: Immutable by definition — a SHA always points to the same content

openvox-code does not implement a separate cache expiration or garbage collection mechanism. To reclaim space from deleted branches or unreferenced objects, you can run `git gc` on the bare clones or simply delete the cache directory and let openvox-code rebuild it on the next mirror.

## Cache and Parallel Operations

During the mirror phase, openvox-code fetches multiple repositories in parallel. Each bare clone has its own lock to prevent concurrent writes to the same repository. Reads (during the deploy phase) do not require locks and can proceed fully in parallel.
