# Build the kube-diagnoser binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY pkg/ pkg/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -mod vendor -o kube-diagnoser main.go

# Use ubuntu as base image to package the kube-diagnoser binary with diagnosing tools
FROM ubuntu:20.04

WORKDIR /usr/bin/
# Copy diagnosing tools
COPY tools/ctr .
COPY tools/docker .

WORKDIR /
# Copy kube-diagnoser binary
COPY --from=builder /workspace/kube-diagnoser .

USER root:root

ENTRYPOINT ["/kube-diagnoser"]
