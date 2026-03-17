# Kubernetes Integration

openvox-code is designed to run natively in Kubernetes as a long-running process.
This removes the need for external CI/CD pipelines to deploy Puppet code — users
can adopt openvox-operator without any existing automation infrastructure.

## Motivation

With openvox-operator, Puppet servers run in Kubernetes. But the Puppet code
(environments and modules) still needs to get there somehow. Traditionally this
requires a CI/CD pipeline, a cron job on a VM, or manual deployment.

openvox-code solves this by running **inside the cluster**, continuously syncing
Puppet environments from Git.

## Deployment Modes

### Mode 1: PVC Sync (simple)

openvox-code runs as a `Deployment` or `CronJob`, syncing environments directly
into a `PersistentVolumeClaim`. The openvox-operator mounts this PVC via
`spec.code.claimName`.

**Pros:** Simple, no registry needed  
**Cons:** PVC must be `ReadWriteMany` for multiple server replicas

### Mode 2: OCI Image Push (recommended)

openvox-code builds an OCI image with the Puppet environments and pushes it to a
container registry. openvox-operator picks up the new image via `spec.code.image`
and rolls it out automatically.

```
Git repo → openvox-code → OCI Registry → openvox-operator → rolling update
```

openvox-operator detects the new image via rollout tracking (`status.configHash`)
and triggers a rolling update across all Server pods — no manual intervention.

**Pros:** Immutable, reproducible, clean rollouts  
**Cons:** Requires a container registry

### Mode 3: Native CRD Integration (future idea)

A potential `CodeSource` CRD managed directly by openvox-operator:

```yaml
apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeSource
metadata:
  name: production
spec:
  configRef: production
  git:
    url: https://git.example.com/puppet/control.git
    branches: ["production", "staging"]
  schedule: "*/5 * * * *"
  output:
    image: ghcr.io/example/puppet-code
```

The operator would spawn openvox-code Jobs internally and update
`Config.spec.code.image` automatically when new commits are detected.

!!! note
    Mode 3 is not yet implemented. Contributions welcome.

## No CI/CD Required

With openvox-code running in-cluster, the full workflow becomes:

1. Developer pushes to Git control repository
2. openvox-code detects new commits (poll or webhook)
3. openvox-code syncs, builds OCI image, pushes to registry
4. openvox-operator detects new image, rolls out to all Puppet servers

**No external CI/CD pipeline needed.**
