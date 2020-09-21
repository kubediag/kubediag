# Build the kube-diagnoser binary
FROM golang:1.14 as builder

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

# Configure apt data sources.
RUN echo "deb http://mirrors.aliyun.com/ubuntu/ focal main restricted" > /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-updates main restricted" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal universe" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-updates universe" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal multiverse" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-updates multiverse" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-backports main restricted universe multiverse" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-security main restricted" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-security universe" >> /etc/apt/sources.list && \
    echo "deb http://mirrors.aliyun.com/ubuntu/ focal-security multiverse" >> /etc/apt/sources.list

# Install utility tools
RUN apt-get update -y && \
    apt-get install -y coreutils dnsutils iputils-ping iproute2 telnet curl vim less wget graphviz && \
    apt-get clean

# Install Go
RUN wget https://golang.org/dl/go1.14.9.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.14.9.linux-amd64.tar.gz && \
    rm go1.14.9.linux-amd64.tar.gz

WORKDIR /usr/bin/
# Copy diagnosing tools
COPY tools/ctr .
COPY tools/docker .

WORKDIR /
# Copy kube-diagnoser binary
COPY --from=builder /workspace/kube-diagnoser .

ENV PATH=$PATH:/usr/local/go/bin

USER root:root

ENTRYPOINT ["/kube-diagnoser"]
