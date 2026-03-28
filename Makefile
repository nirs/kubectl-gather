# SPDX-FileCopyrightText: The kubectl-gather authors
# SPDX-License-Identifier: Apache-2.0

# 0.5.1 when building from tag (release)
# 0.5.1-1-gcf79160 when building without tag (development)
version := $(shell git describe --tags | sed -e 's/^v//')

REGISTRY ?= ghcr.io
REPO ?= nirs
IMAGE ?= gather
TAG ?= $(version)
GOARCH ?= $(shell go env GOARCH)
PLATFORMS ?= linux/amd64,linux/arm64

package := github.com/nirs/kubectl-gather/pkg/gather

image := $(REGISTRY)/$(REPO)/$(IMAGE):$(TAG)
image_arch = $(image)-$(GOARCH)

# Use the toolchain version for the container base image so go build inside
# the container does not need to download a newer toolchain.
go_version = $(shell go mod edit -json | jq -r .Toolchain | sed 's/^go//')

commit := $(shell git rev-parse HEAD)

# % go build -ldflags="-help"
#  -s	disable symbol table
#  -w	disable DWARF generation
#  -X 	definition
#    	add string value definition of the form importpath.name=value
ldflags := -s -w \
	-X '$(package).Version=$(version)' \
	-X '$(package).Commit=$(commit)' \
	-X '$(package).Image=$(image)'

.PHONY: \
	all \
	kubectl-gather \
	lint \
	test \
	clean \
	e2e-build \
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

test: e2e-build e2e-deploy e2e-container
	rm -rf e2e/out/test-*
	cd e2e && go test . -v -count=1

clean:
	cd e2e && go run ./cmd delete
	rm -rf e2e/out

# Build container image locally. Uses qemu emulation for non-native platforms.
container:
	podman build \
		--platform=$(PLATFORMS) \
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

# Use a fixed "devel" image tag and minimal ldflags to maximize container
# build cache reuse. The go defaults in gather.go match these values, so
# no -X flags are needed.
e2e_image := $(REGISTRY)/$(REPO)/$(IMAGE):devel
e2e_ldflags := -s -w

# Build kubectl-gather with default "devel" version for testing.
e2e-build:
	GO_TOOLCHAIN=auto CGO_ENABLED=0 go build -ldflags="$(e2e_ldflags)"

e2e-clusters:
	cd e2e && go run ./cmd create

e2e-deploy: e2e-clusters
	kubectl apply -k e2e/testdata/common --context c1
	kubectl apply -k e2e/testdata/common --context c2
	kubectl apply -k e2e/testdata/c1 --context c1
	kubectl apply -k e2e/testdata/c2 --context c2
	kubectl rollout status deploy common-busybox -n test-common --context c1
	kubectl rollout status deploy common-busybox -n test-common --context c2
	kubectl rollout status deploy c1-busybox -n test-c1 --context c1
	kubectl rollout status deploy c2-busybox -n test-c2 --context c2

e2e-undeploy: e2e-clusters
	kubectl delete -k e2e/testdata/common --context c1 --ignore-not-found --wait=false
	kubectl delete -k e2e/testdata/common --context c2 --ignore-not-found --wait=false
	kubectl delete -k e2e/testdata/c1 --context c1 --ignore-not-found --wait=false
	kubectl delete -k e2e/testdata/c2 --context c2 --ignore-not-found --wait=false
	kubectl wait ns test-common --for delete --context c1
	kubectl wait ns test-common --for delete --context c2
	kubectl wait ns test-c1 --for delete --context c1
	kubectl wait ns test-c2 --for delete --context c2

# Build native container image and load it into e2e clusters.
e2e-container: e2e-clusters
	podman build \
		--tag $(e2e_image) \
		--build-arg ldflags="$(e2e_ldflags)" \
		--build-arg go_version="$(go_version)" \
		.
	mkdir -p e2e/out
	podman save -o e2e/out/gather.tar $(e2e_image)
	cd e2e && go run ./cmd load out/gather.tar
