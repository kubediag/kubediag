IMG ?= hub.c.163.com/kubediag/http-operation
TAG ?= $(shell git rev-parse --short HEAD)

all: http-operation

http-operation: fmt vet
	go build -mod vendor -o http-operation

fmt:
	go fmt ./...

vet:
	go vet ./...

docker-build:
	docker build . -t ${IMG}:${TAG}

docker-push:
	docker push ${IMG}:${TAG}
