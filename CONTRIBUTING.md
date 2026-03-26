# Contributing

## Setting up development environment

### Fedora

Install required packages:

```console
sudo dnf install git golang make podman
```
Check required go version in go.mod. If your distro version is too old, see
[Managing Go installations](https://go.dev/doc/manage-install) for info on
installing the required version.

Install additional tools:

- *kubectl*: https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/
- *kind*: https://kind.sigs.k8s.io/docs/user/quick-start/
- *oc*: https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable/

### macOS

Install the require packages:

```console
brew install git go make podman kubectl kind
```

To build container images or run tests using kind you need to start the podman machine:

```console
podman machine init
podman machine start
```

Install additional tools:

- *oc*: https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable/

### Get the source

Fork the project in github and clone the source:

```console
git clone https://github.com/{my-github-username}/kubectl-gather.git
```

## Build

```console
make
```

## Installing

Install a symlink to `kubectl-gather` and `kubectl_complete-gather` in
the PATH so *kubectl* and *oc* can find them.

```console
ln -s $PWD/kubectl-gather ~/bin/
ln -s $PWD/kubectl_complete-gather ~/bin/
ln -s $PWD/kubectl_complete-gather ~/bin/oc_complete-gather
```

## Creating test clusters

Create test clusters:

```console
kind create cluster -n c1
kind create cluster -n c2
```

> [!NOTE]
> kind adds "kind-" prefix to the cluster names

## Testing local gather

Gather data from the kind clusters:

```console
% kubectl gather --contexts kind-c1,kind-c2 -d gather.local
2024-09-04T02:05:12.146+0300	INFO	gather	Using kubeconfig "/Users/nsoffer/.kube/config"
2024-09-04T02:05:12.148+0300	INFO	gather	Gathering from all namespaces
2024-09-04T02:05:12.148+0300	INFO	gather	Using all addons
2024-09-04T02:05:12.148+0300	INFO	gather	Gathering from cluster "kind-c1"
2024-09-04T02:05:12.148+0300	INFO	gather	Gathering from cluster "kind-c2"
2024-09-04T02:05:12.288+0300	INFO	gather	Gathered 324 resources from cluster "kind-c1" in 0.140 seconds
2024-09-04T02:05:12.288+0300	INFO	gather	Gathered 339 resources from cluster "kind-c2" in 0.140 seconds
2024-09-04T02:05:12.288+0300	INFO	gather	Gathered 663 resources from 2 clusters in 0.140 seconds
```

Inspecting gathered data:

```console
% tree -L2 gather.local
gather.local
├── gather.log
├── kind-c1
│   ├── cluster
│   └── namespaces
└── kind-c2
    ├── cluster
    └── namespaces
```

## Build a container image

Build a multi-arch container image locally for testing:

```console
make container
```

This builds for both `linux/amd64` and `linux/arm64` using QEMU
emulation for the non-native platform. To build for a single
platform:

```console
make container PLATFORMS=linux/arm64
```

> [!NOTE]
> QEMU emulation may not work with all Go versions. Go 1.26 is known
> to crash under QEMU user-mode emulation. Use `PLATFORMS` to build
> only for your native architecture as a workaround.

## Push a container image

The release workflow pushes the container image to ghcr.io automatically.
You should not need to push manually. If you do, create a GitHub Personal
Access Token (classic) with `write:packages` scope. Fine-grained tokens
do not support GitHub Packages yet.

Create a token at https://github.com/settings/tokens/new?scopes=write:packages

1. Login to ghcr.io using the PAT as the password:

   ```console
   podman login ghcr.io -u my-github-user
   ```

2. Push to your ghcr.io repo:

   ```console
   make container-push REPO=my-github-user
   ```

3. If this is your first push, make the package public so
   `kubectl-gather --remote` can pull it. GHCR packages are private
   by default. Change the visibility at
   `https://github.com/users/<username>/packages/container/gather/settings`.

## Testing remote gather

Gather data data remotely using your new image:

```console
% kubectl gather --contexts kind-c1,kind-c2 --remote -d gather.remote
2024-09-04T02:45:26.051+0300	INFO	gather	Using kubeconfig "/Users/nsoffer/.kube/config"
2024-09-04T02:45:26.051+0300	INFO	gather	Gathering from all namespaces
2024-09-04T02:45:26.051+0300	INFO	gather	Using all addons
2024-09-04T02:45:26.051+0300	INFO	gather	Gathering on remote cluster "kind-c2"
2024-09-04T02:45:26.052+0300	INFO	gather	Gathering on remote cluster "kind-c1"
2024-09-04T02:45:36.435+0300	INFO	gather	Gathered on remote cluster "kind-c2" in 10.383 seconds
2024-09-04T02:45:36.437+0300	INFO	gather	Gathered on remote cluster "kind-c1" in 10.385 seconds
2024-09-04T02:45:36.437+0300	INFO	gather	Gathered 2 clusters in 10.385 seconds
```

Inspecting gathered data:

```console
% tree -L3 gather.remote
gather.remote
├── gather.log
├── kind-c1
│   ├── event-filter.html
│   ├── must-gather.log
│   ├── must-gather.logs
│   ├── ghcr-io-nirs-gather-sha256-aa5b3469396e5fc9217a4ffb2cc88465c4dedb311aef072bc1556b2a34f1339c
│   │   ├── cluster
│   │   ├── gather.log
│   │   ├── gather.logs
│   │   ├── namespaces
│   │   └── version
│   └── timestamp
└── kind-c2
    ├── event-filter.html
    ├── must-gather.log
    ├── must-gather.logs
    ├── ghcr-io-nirs-gather-sha256-aa5b3469396e5fc9217a4ffb2cc88465c4dedb311aef072bc1556b2a34f1339c
    │   ├── cluster
    │   ├── gather.log
    │   ├── gather.logs
    │   ├── namespaces
    │   └── version
    └── timestamp
```

## Sending pull requests

Pull requests can be submitted to https://github.com/nirs/kubectl-gather/pulls.

Tips:

- Keep pull requests small
- Each commit should have one logical change
- Before sending a pull request rebase on upstream main branch.
- Test your changes with local and remote clusters
