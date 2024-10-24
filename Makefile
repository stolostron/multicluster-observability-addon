# include the bingo binary variables. This enables the bingo versions to be
# referenced here as make variables. For example: $(GOLANGCI_LINT)
include .bingo/Variables.mk

# set the default target here, because the include above will automatically set
# it to the first defined target
.DEFAULT_GOAL := default
default: all

VERSION ?= v0.0.1

CRD_DIR := $(shell pwd)/deploy/crds

# REGISTRY_BASE
# defines the container registry and organization for the bundle and operator container images.
REGISTRY_BASE_OPENSHIFT = quay.io/rhobs
REGISTRY_BASE ?= $(REGISTRY_BASE_OPENSHIFT)

ARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)

# Image URL to use all building/pushing image targets
IMG ?= $(REGISTRY_BASE)/multicluster-observability-addon:$(VERSION)

.PHONY: deps
deps: go.mod go.sum
	go mod tidy
	go mod download
	go mod verify

$(CRD_DIR)/observability.openshift.io_clusterlogforwarders.yaml:
	@mkdir -p $(CRD_DIR)
	@curl https://raw.githubusercontent.com/openshift/cluster-logging-operator/release-6.0/bundle/manifests/observability.openshift.io_clusterlogforwarders.yaml > $(CRD_DIR)/observability.openshift.io_clusterlogforwarders.yaml

$(CRD_DIR)/opentelemetry.io_opentelemetrycollectors.yaml:
	@mkdir -p $(CRD_DIR)
	@curl https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v0.100.1/bundle/manifests/opentelemetry.io_opentelemetrycollectors.yaml > $(CRD_DIR)/opentelemetry.io_opentelemetrycollectors.yaml

$(CRD_DIR)/opentelemetry.io_instrumentations.yaml:
	@mkdir -p $(CRD_DIR)
	@curl https://raw.githubusercontent.com/open-telemetry/opentelemetry-operator/v0.100.1/bundle/manifests/opentelemetry.io_instrumentations.yaml > $(CRD_DIR)/opentelemetry.io_instrumentations.yaml

.PHONY: install-crds
install-crds: $(CRD_DIR)/observability.openshift.io_clusterlogforwarders.yaml $(CRD_DIR)/opentelemetry.io_opentelemetrycollectors.yaml $(CRD_DIR)/opentelemetry.io_instrumentations.yaml

.PHONY: fmt
fmt: $(GOFUMPT) ## Run gofumpt on source code.
	find . -type f -name '*.go' -not -path '**/fake_*.go' -exec $(GOFUMPT) -w {} \;

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint on source code.
	$(GOLANGCI_LINT) run --timeout=5m ./...

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Attempt to automatically fix lint issues in source code.
	$(GOLANGCI_LINT) run --fix --timeout=5m ./...

.PHONY: test
test:
	go test -v ./internal/...

.PHONY: addon
addon: deps fmt ## Build addon binary
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(ARCH) go build -o bin/multicluster-observability-addon_${ARCH} -ldflags "-s -w" main.go

.PHONY: oci-build
oci-build: GOOS = linux
oci-build: addon ## Build the image
	podman build -t ${IMG} .

.PHONY: oci-push
oci-push: ## Push the image
	podman push ${IMG}

.PHONY: oci
oci: oci-build oci-push

.PHONY: addon-deploy
addon-deploy: $(KUSTOMIZE) install-crds
	cd deploy && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build ./deploy | kubectl apply -f -

.PHONY: addon-undeploy
addon-undeploy: $(KUSTOMIZE) install-crds
	$(KUSTOMIZE) build ./deploy | kubectl delete -f -
