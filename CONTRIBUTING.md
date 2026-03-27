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

## Building and pushing container images

Container images are built and published by CI. For local testing with
`--remote`, build and push a multi-arch image to your own registry:

```console
make container REPO=my-quay-user
make container-push
```

> [!IMPORTANT]
> Make your repo public to use it for gathering.

## Sending pull requests

Pull requests can be submitted to https://github.com/nirs/kubectl-gather/pulls.

Tips:

- Keep pull requests small
- Each commit should have one logical change
- Before sending a pull request rebase on upstream main branch
- Test your changes with `make test`
