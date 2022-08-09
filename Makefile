
# Image URL to use all building/pushing image targets
IMG ?= hub.c.163.com/kubediag/kubediag
# Image tag to use all building/pushing image targets
TAG ?= $(shell git rev-parse --short HEAD)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: kubediag

# Run e2e tests
e2e: 
	go test ./test/e2e/... -coverprofile cover.out

# Run unit tests
test: generate fmt vet manifests
	go test ./pkg/... -coverprofile cover.out

# Build kubediag binary
kubediag: generate fmt vet
	go mod vendor
	go build -mod vendor -o bin/kubediag main.go

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
	kubectl apply -f config/deploy

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=kubediag-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cd config/manager && $(KUSTOMIZE) edit set image hub.c.163.com/kubediag/kubediag=${IMG}:${TAG}
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
docker-build: test
	docker build . -t ${IMG}:${TAG}

# Push the docker image
docker-push:
	docker push ${IMG}:${TAG}

# Generate api docs
doc: gen-api-docs
	$(GEN_API_DOCS) \
	-config ./docs/api/gen-crd-api-reference-docs/example-config.json \
	-api-dir  ./api/v1 \
	-out-file ./docs/api/api-docs.md \
	--template-dir ./docs/api/gen-crd-api-reference-docs/template

# Find or download controller-gen
# Download controller-gen if necessary
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
CONTROLLER_GEN=$(shell which controller-gen)
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

PROJECT_DIR = $(shell pwd)
GEN_API_DOCS = $(PROJECT_DIR)/bin/gen-crd-api-reference-docs
gen-api-docs: ## Download gen-crd-api-referenc-docs locally if necessary
ifeq (, $(wildcard $(GEN_API_DOCS)))
	@{ \
	set -e ;\
	GEN_API_DOCS_TMP_DIR=$$(mktemp -d) ;\
	cd $$GEN_API_DOCS_TMP_DIR ;\
	go mod init tmp ;\
	git clone https://github.com/ahmetb/gen-crd-api-reference-docs.git -b v0.3.0 ;\
	cd gen-crd-api-reference-docs ;\
	go build -o $(PROJECT_DIR)/bin/ ;\
	rm -rf $$GEN_API_DOCS_TMP_DIR ;\
	}
endif

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.0.5)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
