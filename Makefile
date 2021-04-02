
# Image URL to use all building/pushing image targets
BOOTSTRAP_IMG ?= ghcr.io/zawachte-msft/cluster-api-k3s/bootstrap-controller:v0.1.2

# Image URL to use all building/pushing image targets
CONTROLPLANE_IMG ?= ghcr.io/zawachte-msft/cluster-api-k3s/controlplane-controller:v0.1.2


# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all-bootstrap: manager-bootstrap

# Run tests
test-bootstrap: generate-bootstrap fmt vet manifests-bootstrap
	go test ./... -coverprofile cover.out

# Build manager binary
manager-bootstrap: generate-bootstrap fmt vet
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o bin/manager bootstrap/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run-bootstrap: generate-bootstrap fmt vet manifests-bootstrap
	go run ./bootstrap/main.go

# Install CRDs into a cluster
install-bootstrap: manifests-bootstrap
	kustomize build bootstrap/config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall-bootstrap: manifests-bootstrap
	kustomize build bootstrap/config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-bootstrap: manifests-bootstrap
	cd bootstrap/config/manager && kustomize edit set image controller=${BOOTSTRAP_IMG}
	kustomize build bootstrap/config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests-bootstrap: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=bootstrap/config/crd/bases

release-bootstrap: manifests-bootstrap
	mkdir -p out
	cd bootstrap/config/manager && kustomize edit set image controller=${BOOTSTRAP_IMG}
	kustomize build bootstrap/config/default > out/bootstrap-components.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate-bootstrap: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build-bootstrap: manager-bootstrap
	docker build . -t ${BOOTSTRAP_IMG}

# Push the docker image
docker-push-bootstrap:
	docker push ${BOOTSTRAP_IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

all-controlplane: manager-controlplane

# Run tests
test-controlplane: generate-controlplane fmt vet manifests-controlplane
	go test ./... -coverprofile cover.out

# Build manager binary
manager-controlplane: generate-controlplane fmt vet
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o bin/manager controlplane/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run-controlplane: generate-controlplane fmt vet manifests-controlplane
	go run ./controlplane/main.go

# Install CRDs into a cluster
install-controlplane: manifests-controlplane
	kustomize build controlplane/config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall-controlplane: manifests-controlplane
	kustomize build controlplane/config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-controlplane: manifests-controlplane
	cd controlplane/config/manager && kustomize edit set image controller=${CONTROLPLANE_IMG}
	kustomize build controlplane/config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests-controlplane: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=controlplane/config/crd/bases

release-controlplane: manifests-controlplane
	mkdir -p out
	cd controlplane/config/manager && kustomize edit set image controller=${CONTROLPLANE_IMG}
	kustomize build controlplane/config/default > out/control-plane-components.yaml

generate-controlplane: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build-controlplane: manager-controlplane
	docker build . -t ${CONTROLPLANE_IMG}

# Push the docker image
docker-push-controlplane:
	docker push ${CONTROLPLANE_IMG}
