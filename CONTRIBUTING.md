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

To build container images or run tests using kind you need to create
a podman machine with enough memory. The default 2GB is not enough
when running kind clusters and building container images:

```console
podman machine init --memory 3072
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

## Testing

Build and run the end-to-end tests:

```console
make
make test
```

This creates kind clusters, deploys test workloads, and runs all tests.
On subsequent runs, existing clusters are reused.

To delete the test clusters and clean up test outputs:

```console
make clean
```

## Build a container image

To build a container image for testing run:

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

## Push a container image to your registry

The release workflow pushes the container image automatically. You
should not need to push manually. If you do, push to your own registry:

1. Build the container for your repo:

   ```console
   make container REPO=my-quay-user
   ```

2. Login to the registry:

   ```console
   podman login quay.io -u my-quay-user
   ```

3. Push to your repo:

   ```console
   make container-push REPO=my-quay-user
   ```

> [!IMPORTANT]
> Make your repo public so `kubectl-gather --remote` can pull the image.

## Sending pull requests

Pull requests can be submitted to https://github.com/nirs/kubectl-gather/pulls.

Tips:

- Keep pull requests small
- Each commit should have one logical change
- Before sending a pull request rebase on upstream main branch
- Test your changes with `make test`
