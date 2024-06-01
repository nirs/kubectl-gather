# SPDX-FileCopyrightText: The kubectl-gather authors
# SPDX-License-Identifier: Apache-2.0

REGITRY ?= quay.io
REPO ?= nirsof
IMAGE ?= gather
TAG ?= 0.2

image := $(REGITRY)/$(REPO)/$(IMAGE):$(TAG)

.PHONY: all kubectl-gather

all: kubectl-gather

container:
	podman build -t $(image) .

container-push: container
	podman push $(image)

kubectl-gather:
	CGO_ENABLED=0 go build
