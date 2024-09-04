# SPDX-FileCopyrightText: The kubectl-gather authors
# SPDX-License-Identifier: Apache-2.0

REGISTRY ?= quay.io
REPO ?= nirsof
IMAGE ?= gather
TAG ?= 0.6.0-dev

image := $(REGISTRY)/$(REPO)/$(IMAGE):$(TAG)

# % go build -ldflags="-help"
#  -s	disable symbol table
#  -w	disable DWARF generation
ldflags := -s -w

.PHONY: all kubectl-gather

all: kubectl-gather

container:
	podman build \
		--platform=linux/amd64,linux/arm64 \
		--manifest $(image) \
		--build-arg ldflags="$(ldflags)" \
		.

container-push: container
	podman manifest push --all $(image)

kubectl-gather:
	CGO_ENABLED=0 go build -ldflags="$(ldflags)"
