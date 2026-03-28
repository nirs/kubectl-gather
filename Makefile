# SPDX-FileCopyrightText: The kubectl-gather authors
# SPDX-License-Identifier: Apache-2.0

# 0.5.1 when building from tag (release)
# 0.5.1-1-gcf79160 when building without tag (development)
version := $(shell git describe --tags | sed -e 's/^v//')

REGISTRY ?= quay.io
REPO ?= nirsof
IMAGE ?= gather
TAG ?= $(version)
GOARCH ?= $(shell go env GOARCH)

package := github.com/nirs/kubectl-gather/pkg/gather

image := $(REGISTRY)/$(REPO)/$(IMAGE):$(TAG)
image_arch = $(image)-$(GOARCH)

# Use the toolchain version for the container base image so go build inside
# the container does not need to download a newer toolchain.
go_version = $(shell go mod edit -json | jq -r .Toolchain | sed 's/^go//')

# % go build -ldflags="-help"
#  -s	disable symbol table
#  -w	disable DWARF generation
#  -X 	definition
#    	add string value definition of the form importpath.name=value
ldflags := -s -w \
	-X '$(package).Version=$(version)' \
	-X '$(package).Image=$(image)'

.PHONY: \
	all \
	kubectl-gather \
	lint \
	test \
	clean \
	e2e-clusters \
	e2e-container \
	e2e-deploy \
	e2e-undeploy \
	container \
	container-native \
	container-multiarch \
	container-push \
	container-multiarch-push

all: kubectl-gather

lint:
	golangci-lint run ./...
	cd e2e && golangci-lint run ./...

test: kubectl-gather e2e-deploy e2e-container
	cd e2e && go test . -v -count=1

clean:
	cd e2e && go run ./cmd delete
	rm -rf e2e/out
	# Each build creates a new tagged image; remove all of them and prune
	# the dangling intermediate layers.
	podman images --quiet $(REGISTRY)/$(REPO)/$(IMAGE) | xargs --no-run-if-empty podman rmi --force
	podman image prune --force

# Build multiarch image locally using qemu emulation.
container:
	podman build \
		--platform=linux/amd64,linux/arm64 \
		--manifest $(image) \
		--build-arg ldflags="$(ldflags)" \
		--build-arg go_version="$(go_version)" \
		.

container-push:
	podman manifest push --all $(image)

# Parallel native container build targets used by github workflow.

# Build image for the native architecture.
container-native:
	podman build \
		--tag $(image_arch) \
		--build-arg ldflags="$(ldflags)" \
		--build-arg go_version="$(go_version)" \
		.

# Save native image as OCI archive.
$(IMAGE)-$(GOARCH).tar: container-native
	podman save --format oci-archive -o $@ $(image_arch)

# Create multiarch manifest from per-arch images.
# Use containers-storage: to reference local images, otherwise podman
# tries to pull from the registry.
container-multiarch:
	podman manifest create $(image) \
		containers-storage:$(image)-amd64 \
		containers-storage:$(image)-arm64

# Save multiarch manifest as OCI archive.
$(IMAGE).tar:
	podman manifest push --all $(image) oci-archive:$@

# Push multiarch OCI archive to registry.
container-multiarch-push:
	skopeo copy --all oci-archive:$(IMAGE).tar docker://$(image)

# Build env variables:
# - CGO_ENABLED=0: Disable CGO to avoid dependencies on libc. Built image can
#   be built on latest Fedora and run on old RHEL.
# - GOTOOLCHAIN=auto: The go command downloads newer toolchain as needed.
#   https://go.dev/doc/toolchain#download
kubectl-gather:
	GO_TOOLCHAIN=auto CGO_ENABLED=0 go build -ldflags="$(ldflags)"

# End-to-end test targets.

e2e-deploy e2e-undeploy: export KUBECONFIG = e2e/out/kubeconfig.yaml

e2e-clusters:
	cd e2e && go run ./cmd create

e2e-deploy: e2e-clusters
	kubectl apply -k e2e/testdata/common --context kind-c1
	kubectl apply -k e2e/testdata/common --context kind-c2
	kubectl apply -k e2e/testdata/c1 --context kind-c1
	kubectl apply -k e2e/testdata/c2 --context kind-c2
	kubectl rollout status deploy common-busybox -n test-common --context kind-c1
	kubectl rollout status deploy common-busybox -n test-common --context kind-c2
	kubectl rollout status deploy c1-busybox -n test-c1 --context kind-c1
	kubectl rollout status deploy c2-busybox -n test-c2 --context kind-c2

e2e-undeploy: e2e-clusters
	kubectl delete -k e2e/testdata/common --context kind-c1 --ignore-not-found --wait=false
	kubectl delete -k e2e/testdata/common --context kind-c2 --ignore-not-found --wait=false
	kubectl delete -k e2e/testdata/c1 --context kind-c1 --ignore-not-found --wait=false
	kubectl delete -k e2e/testdata/c2 --context kind-c2 --ignore-not-found --wait=false
	kubectl wait ns test-common --for delete --context kind-c1
	kubectl wait ns test-common --for delete --context kind-c2
	kubectl wait ns test-c1 --for delete --context kind-c1
	kubectl wait ns test-c2 --for delete --context kind-c2

# Build native container image and load it into e2e kind clusters.
e2e-container: e2e-clusters
	podman build \
		--tag $(image) \
		--build-arg ldflags="$(ldflags)" \
		--build-arg go_version="$(go_version)" \
		.
	podman save -o e2e/out/$(IMAGE)-kind.tar $(image)
	cd e2e && go run ./cmd load out/$(IMAGE)-kind.tar
