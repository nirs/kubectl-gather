# SPDX-FileCopyrightText: The kubectl-gather authors
# SPDX-License-Identifier: Apache-2.0

ARG go_version
ARG ldflags

FROM docker.io/library/golang:${go_version} as builder

WORKDIR /build

COPY go.mod go.sum ./

# Cache dependencies before copying sources so source changes do not
# invalidated cacehd dependencies.
RUN go mod download

COPY cmd cmd
COPY pkg pkg
COPY main.go main.go

 # Disable CGO to avoid dependencies on libc. Built image can be built on latest
 # Fedora and run on old RHEL.
RUN CGO_ENABLED=0 go build -ldflags="${ldflags}"

FROM docker.io/library/alpine:latest

# Required for must-gather: rsync, bash
# Required for kubectl-gather: tar, kubectl
RUN apk add --no-cache \
        bash \
        rsync \
        tar \
    && apk add --no-cache --repository=http://dl-cdn.alpinelinux.org/alpine/edge/community \
        kubectl \
    && mkdir -p licenses

COPY --from=builder /build/kubectl-gather /usr/bin/kubectl-gather
COPY gather /usr/bin/gather
COPY LICENSE licenses/Apache-2.0.txt

# Use exec form to allow passing arguemnts from docker commmand.
ENTRYPOINT ["/usr/bin/gather"]
