# SPDX-FileCopyrightText: The kubectl-gather authors
# SPDX-License-Identifier: Apache-2.0

# Build using 3 stages to create a minimal image with Red Hat UBI
# components, enabling security scanning on quay.io:
#
# 1. golang       - build the kubectl-gather binary
# 2. ubi-minimal  - install RPM packages into a separate --installroot
#                   and download kubectl (ubi-micro has no package manager)
# 3. ubi-micro    - copy the install root and binaries into the final image

# Stage 1: Build kubectl-gather

ARG go_version

FROM docker.io/library/golang:${go_version} AS builder

WORKDIR /build

COPY go.mod go.sum ./

# Cache dependencies before copying sources so source changes do not
# invalidate cached dependencies.
RUN go mod download

COPY cmd cmd
COPY pkg pkg
COPY main.go main.go

ARG ldflags

# Build env variables:
# - CGO_ENABLED=0: Disable CGO to avoid dependencies on libc. The binary can be
#   built on latest Fedora and run on old RHEL.
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="${ldflags}"

# Stage 2: Install dependencies into /install-root

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest AS base

# microdnf requires os-release in the install root.
RUN mkdir -p /install-root/etc && \
    cp /etc/os-release /install-root/etc/os-release

# Install packages into /install-root for copying into the final
# ubi-micro image. microdnf requires --noplugins, --config, and --setopt
# for cachedir/reposdir/varsdir when using --installroot.
# --setopt=install_weak_deps=0 avoids optional dependencies to minimize
# the install size.
#
# Packages:
# - bash: required by the gather script
# - tar, gzip: required by kubectl-gather for streaming files from pods
# - rsync: required by oc adm must-gather to copy data from the pod
# - util-linux-core: provides setsid -w, required by must-gather
#
# Removals:
# - Remove glibc locale conversion modules (~20 MB), not needed.
RUN microdnf install -y \
        --installroot=/install-root \
        --noplugins \
        --config=/etc/dnf/dnf.conf \
        --setopt=cachedir=/var/cache/dnf \
        --setopt=reposdir=/etc/yum.repos.d \
        --setopt=varsdir=/etc/dnf/vars \
        --setopt=install_weak_deps=0 \
        --releasever=9 \
        --nodocs \
        bash tar gzip rsync util-linux-core \
    && microdnf clean all \
    && rm -rf /install-root/usr/lib64/gconv

# Download upstream kubectl (smaller than OpenShift's oc/kubectl bundle).
ARG TARGETARCH
RUN KUBE_VERSION=$(curl -fsSL https://dl.k8s.io/release/stable.txt) && \
    curl -fSLo /install-root/usr/bin/kubectl \
        "https://dl.k8s.io/release/${KUBE_VERSION}/bin/linux/${TARGETARCH}/kubectl" && \
    chmod +x /install-root/usr/bin/kubectl

# Stage 3: Final image

FROM registry.access.redhat.com/ubi9/ubi-micro:latest

COPY --from=base /install-root /
COPY --from=builder /build/kubectl-gather /usr/bin/kubectl-gather
COPY gather /usr/bin/gather
COPY LICENSE licenses/Apache-2.0.txt

LABEL org.opencontainers.image.source=https://github.com/nirs/kubectl-gather

ENTRYPOINT ["/usr/bin/gather"]
