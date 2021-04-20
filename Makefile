
# Image URL to use all building/pushing image targets
IMG ?= hub.c.163.com/combk8s/kube-diagnoser
# Image tag to use all building/pushing image targets
TAG ?= $(shell git rev-parse --short HEAD)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Use local kustomize (version v4.0.5)
MKFILE_PATH = $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR = $(dir $(MKFILE_PATH))
KUSTOMIZE = "$(MKFILE_DIR)tools/kustomize"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: kube-diagnoser

# Run e2e tests
e2e-test: 
	go test ./test/e2e/... -coverprofile cover.out

# Run unit tests
unit-test: generate fmt vet manifests
	go test ./pkg/... -coverprofile cover.out

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build kube-diagnoser binary
kube-diagnoser: generate fmt vet
	go mod vendor
	go build -mod vendor -o bin/kube-diagnoser main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && $(KUSTOMIZE) edit set image hub.c.163.com/combk8s/kube-diagnoser=${IMG}:${TAG}
	$(KUSTOMIZE) build config/default > config/deploy/manifests.yaml
	kubectl apply -f config/deploy

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=kube-diagnoser-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(KUSTOMIZE) build config/default > config/deploy/manifests.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: unit-test
	docker build . -t ${IMG}:${TAG}

# Push the docker image
docker-push:
	docker push ${IMG}:${TAG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
