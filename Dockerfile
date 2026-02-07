# Multi-stage build for optimized image size
FROM golang:1.24 AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o packet-capture-controller \
    ./cmd/controller

FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        tcpdump \
        util-linux \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /workspace/packet-capture-controller /usr/local/bin/

RUN mkdir -p /var/log/antrea-captures

USER root

ENTRYPOINT ["/usr/local/bin/packet-capture-controller"]
