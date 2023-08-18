# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If you update this file, please follow
# https://www.thapaliya.com/en/writings/well-documented-makefiles/

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

GO_VERSION ?= 1.20.6
GO_CONTAINER_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

ARCH ?= $(shell go env GOARCH)

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

GO_INSTALL := ./hack/go_install.sh

BIN_DIR := bin
TOOLS_BIN_DIR := $(shell pwd)/$(BIN_DIR)
$(TOOLS_BIN_DIR):
	mkdir -p $(TOOLS_BIN_DIR)

# Image URL to use all building/pushing image targets
BOOTSTRAP_IMG_TAG ?= v0.2.0
BOOTSTRAP_IMG ?= ghcr.io/cluster-api-provider-k3s/cluster-api-k3s/bootstrap-controller:v0.2.0

# Image URL to use all building/pushing image targets
CONTROLPLANE_IMG_TAG ?= v0.2.0
CONTROLPLANE_IMG ?= ghcr.io/cluster-api-provider-k3s/cluster-api-k3s/controlplane-controller:$(CONTROLPLANE_IMG_TAG)


# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Sync to controller-tools version in https://github.com/kubernetes-sigs/cluster-api/blob/v{VERSION}/hack/tools/go.mod
CONTROLLER_GEN_VER := v0.12.1
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER)

# Sync to github.com/drone/envsubst/v2 in https://github.com/kubernetes-sigs/cluster-api/blob/v{VERSION}/go.mod
ENVSUBST_VER := v2.0.0-20210730161058-179042472c46
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)

ENVTEST_VER := latest
ENVTEST_BIN := setup-envtest
ENVTEST := $(TOOLS_BIN_DIR)/$(ENVTEST_BIN)
ENVTEST_K8S_VERSION = "1.26.0"

# Sync to github.com/drone/envsubst/v2 in https://github.com/kubernetes-sigs/cluster-api/blob/v{VERSION}/go.mod
ENVSUBST_VER := v2.0.0-20210730161058-179042472c46
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)

# Bump as necessary/desired to latest that supports our version of go at https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VER := v1.53.3
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER)

# Keep at 4.0.4 until we figure out how to get later verisons to not mangle the calico yamls
# HACK bump latest version once https://github.com/kubernetes-sigs/kustomize/issues/947 is fixed
KUSTOMIZE_VER := v4.0.4
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER)


all-bootstrap: manager-bootstrap

# Run tests
test-bootstrap: envtest generate-bootstrap lint manifests-bootstrap
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLS_BIN_DIR) -p path)" go test $(shell pwd)/bootstrap/... -coverprofile cover.out

# Build manager binary
manager-bootstrap: generate-bootstrap lint
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o bin/manager bootstrap/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run-bootstrap: generate-bootstrap lint manifests-bootstrap
	go run ./bootstrap/main.go

# Install CRDs into a cluster
install-bootstrap: manifests-bootstrap
	$(KUSTOMIZE) build bootstrap/config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall-bootstrap: manifests-bootstrap
	$(KUSTOMIZE) build bootstrap/config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-bootstrap: manifests-bootstrap
	cd bootstrap/config/manager && $(KUSTOMIZE) edit set image controller=${BOOTSTRAP_IMG}
	$(KUSTOMIZE) build bootstrap/config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests-bootstrap: $(KUSTOMIZE) $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=bootstrap/config/crd/bases output:rbac:dir=bootstrap/config/rbac

release-bootstrap: manifests-bootstrap ## Release bootstrap
	mkdir -p out
	cd bootstrap/config/manager && $(KUSTOMIZE) edit set image controller=${BOOTSTRAP_IMG}
	$(KUSTOMIZE) build bootstrap/config/default > out/bootstrap-components.yaml

# Generate code
generate-bootstrap: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="$(shell pwd)/bootstrap/..."

# Build the docker image
docker-build-bootstrap: manager-bootstrap ## Build bootstrap
	DOCKER_BUILDKIT=1 docker build --build-arg builder_image=$(GO_CONTAINER_IMAGE) --build-arg goproxy=$(GOPROXY) --build-arg ARCH=$(ARCH) --build-arg package=./bootstrap/main.go --build-arg ldflags="$(LDFLAGS)" . -t ${BOOTSTRAP_IMG}

# Push the docker image
docker-push-bootstrap: ## Push bootstrap
	docker push ${BOOTSTRAP_IMG}

all-controlplane: manager-controlplane

# Run tests
test-controlplane: envtest generate-controlplane lint manifests-controlplane
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(TOOLS_BIN_DIR) -p path)" go test $(shell pwd)/controlplane/... -coverprofile cover.out

# Build manager binary
manager-controlplane: generate-controlplane lint
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o bin/manager controlplane/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run-controlplane: generate-controlplane lint manifests-controlplane
	go run ./controlplane/main.go

# Install CRDs into a cluster
install-controlplane: manifests-controlplane
	$(KUSTOMIZE) build controlplane/config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall-controlplane: manifests-controlplane
	$(KUSTOMIZE) build controlplane/config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-controlplane: manifests-controlplane
	cd controlplane/config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLPLANE_IMG}
	$(KUSTOMIZE) build controlplane/config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests-controlplane: $(KUSTOMIZE) $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) rbac:roleName=manager-role webhook crd paths="./..." output:crd:artifacts:config=controlplane/config/crd/bases output:rbac:dir=controlplane/config/rbac

release-controlplane: manifests-controlplane ## Release control-plane
	mkdir -p out
	cd controlplane/config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLPLANE_IMG}
	$(KUSTOMIZE) build controlplane/config/default > out/control-plane-components.yaml

generate-controlplane: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="$(shell pwd)/controlplane/..." 

docker-build-controlplane: manager-controlplane ## Build control-plane
	DOCKER_BUILDKIT=1 docker build --build-arg builder_image=$(GO_CONTAINER_IMAGE) --build-arg goproxy=$(GOPROXY) --build-arg ARCH=$(ARCH) --build-arg package=./controlplane/main.go --build-arg ldflags="$(LDFLAGS)" . -t ${CONTROLPLANE_IMG}

docker-push-controlplane: ## Push control-plane
	docker push ${CONTROLPLANE_IMG}

release: release-bootstrap release-controlplane
## --------------------------------------
## Help
## --------------------------------------

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v -c .golangci.yml

fmt:
	go fmt ./...


## --------------------------------------
## Tooling Binaries
## --------------------------------------
.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(TOOLS_BIN_DIR)
	test -s $(TOOLS_BIN_DIR)/setup-envtest || GOBIN=$(TOOLS_BIN_DIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

$(ENVSUBST): ## Build envsubst from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/drone/envsubst/v2/cmd/envsubst $(ENVSUBST_BIN) $(ENVSUBST_VER)

$(GOLANGCI_LINT): ## Build golangci-lint from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)

## HACK replace with $(GO_INSTALL) once https://github.com/kubernetes-sigs/kustomize/issues/947 is fixed
$(KUSTOMIZE): ## Put kustomize into tools folder.
	mkdir -p $(TOOLS_BIN_DIR)
	rm -f $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)*
	curl -fsSL "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash -s -- $(KUSTOMIZE_VER:v%=%) $(TOOLS_BIN_DIR)
	mv "$(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)" $(KUSTOMIZE)
	ln -sf $(KUSTOMIZE) "$(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)"

$(CONTROLLER_GEN): ## Build controller-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) sigs.k8s.io/controller-tools/cmd/controller-gen $(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN_VER)
