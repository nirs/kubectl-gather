# SPDX-FileCopyrightText: The kubectl-gather authors
# SPDX-License-Identifier: Apache-2.0

REGISTRY ?= quay.io
REPO ?= nirsof
IMAGE ?= gather

package := github.com/nirs/kubectl-gather/pkg/gather

# 0.5.1 when building from tag (release)
# 0.5.1-1-gcf79160 when building without tag (development)
version := $(shell git describe --tags | sed -e 's/^v//')

image := $(REGISTRY)/$(REPO)/$(IMAGE):$(version)

# % go build -ldflags="-help"
#  -s	disable symbol table
#  -w	disable DWARF generation
#  -X 	definition
#    	add string value definition of the form importpath.name=value
ldflags := -s -w \
	-X '$(package).Version=$(version)' \
	-X '$(package).Image=$(image)'

.PHONY: all kubectl-gather

all: kubectl-gather

lint:
	golangci-lint run ./...
	cd e2e && golangci-lint run ./...

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
