# SPDX-FileCopyrightText: The kubectl-gather authors
# SPDX-License-Identifier: Apache-2.0

REGISTRY ?= quay.io
REPO ?= nirsof
IMAGE ?= gather
TAG ?= 0.5.1

image := $(REGISTRY)/$(REPO)/$(IMAGE):$(TAG)

# % go build -ldflags="-help"
#  -s	disable symbol table
#  -w	disable DWARF generation
ldflags := -s -w

.PHONY: all kubectl-gather

all: kubectl-gather

container:
	podman build --tag $(image) --build-arg ldflags="$(ldflags)" .

container-push: container
	podman push $(image)

kubectl-gather:
	CGO_ENABLED=0 go build -ldflags="$(ldflags)"
