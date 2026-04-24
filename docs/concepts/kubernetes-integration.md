# Kubernetes Integration

openvox-code is a CLI tool that runs in CI/CD pipelines. It builds OCI images
containing Puppet environments and pushes them to a container registry. The
[openvox-operator](https://github.com/slauger/openvox-operator) (a separate
project) runs in Kubernetes and consumes these images.

## Workflow

The typical workflow looks like this:

1. Developer pushes to the Git control repository
2. CI/CD pipeline runs `openvox-code sync` and `openvox-code build`
3. openvox-code builds an OCI image with all Puppet environments and pushes it to a container registry
4. openvox-operator detects the new image and rolls it out to all Puppet servers in the cluster

```
Git repo → CI/CD pipeline → openvox-code → OCI Registry → openvox-operator → rolling update
```

## OCI Image Output

openvox-code builds an OCI image with the Puppet environments and pushes it to a
container registry. openvox-operator picks up the new image via `spec.code.image`
and rolls it out automatically.

openvox-operator detects the new image via rollout tracking (`status.configHash`)
and triggers a rolling update across all Server pods — no manual intervention.

## CI/CD Pipeline Example

```bash
# Sync environments locally
openvox-code sync --config openvox-code.yaml

# Build and push OCI image
openvox-code build --config openvox-code.yaml --registry ghcr.io/example/puppet-envs --tag v1.0.0 --push
```

## Future: Native CRD Integration

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
    This mode is not yet implemented. Contributions welcome.
