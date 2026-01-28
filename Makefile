# include the bingo binary variables. This enables the bingo versions to be
# referenced here as make variables. For example: $(GOLANGCI_LINT)
include .bingo/Variables.mk

# set the default target here, because the include above will automatically set
# it to the first defined target
.DEFAULT_GOAL := default
default: all

VERSION ?= v0.0.1

CRD_DIR := $(shell pwd)/deploy/crds
BIN_DIR := $(CURDIR)/bin
CONTAINER_ENGINE ?= $(shell which podman 2>/dev/null || which docker 2>/dev/null)

# REGISTRY_BASE
# defines the container registry and organization for the bundle and operator container images.
REGISTRY_BASE_OPENSHIFT = quay.io/rhobs
REGISTRY_BASE ?= $(REGISTRY_BASE_OPENSHIFT)

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

$(CRD_DIR)/monitoring.coreos.com_prometheusagents.yaml:
	@mkdir -p $(CRD_DIR)
	@curl https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/refs/tags/v0.80.1-rhobs1/example/prometheus-operator-crd/monitoring.rhobs_prometheusagents.yaml  > $(CRD_DIR)/monitoring.rhobs_prometheusagents.yaml

$(CRD_DIR)/monitoring.coreos.com_scrapeconfigs.yaml:
	@mkdir -p $(CRD_DIR)
	@curl https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/refs/tags/v0.80.1-rhobs1/example/prometheus-operator-crd/monitoring.rhobs_scrapeconfigs.yaml  > $(CRD_DIR)/monitoring.rhobs_scrapeconfigs.yaml

.PHONY: update-metrics-crds
update-metrics-crds: ## Update the metrics CRDs from the rhobs/obo-prometheus-operator repository.
	@./hack/update-metrics-crds.sh

.PHONY: download-crds
download-crds: $(CRD_DIR)/observability.openshift.io_clusterlogforwarders.yaml $(CRD_DIR)/opentelemetry.io_opentelemetrycollectors.yaml $(CRD_DIR)/opentelemetry.io_instrumentations.yaml $(CRD_DIR)/monitoring.coreos.com_prometheusagents.yaml $(CRD_DIR)/monitoring.coreos.com_scrapeconfigs.yaml

.PHONY: fmt
fmt: $(GOFUMPT) ## Run gofumpt on source code.
	find . -type f -name '*.go' -not -path '**/fake_*.go' -exec $(GOFUMPT) -w {} \;

.PHONY: verify-dockerfile-labels
verify-dockerfile-labels: ## Verify Dockerfile.Konflux RHEL version consistency
	@./hack/verify-dockerfile-labels.sh

.PHONY: lint
lint: $(GOLANGCI_LINT) verify-dockerfile-labels ## Run golangci-lint and dockerfile verification
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run --timeout=5m ./...

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Attempt to automatically fix lint issues in source code.
	$(GOLANGCI_LINT) run --fix --timeout=5m ./...

.PHONY: test
test:
	go test ./internal/...

.PHONY: prepare-bin
prepare-bin:
	@mkdir -p $(BIN_DIR)

.PHONY: addon
addon: deps fmt ## Build addon binary
	go build -o bin/multicluster-observability-addon main.go

.PHONY: oci-build
oci-build: ## Build the image
	$(CONTAINER_ENGINE) build -t ${IMG} .

.PHONY: oci-push
oci-push: ## Push the image
	$(CONTAINER_ENGINE) push ${IMG}

.PHONY: oci
oci: oci-build oci-push

.PHONY: install-crds
install-crds: download-crds
	kubectl apply --server-side -f $(CRD_DIR)

.PHONY: addon-deploy
addon-deploy: download-crds
	cd deploy && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build ./deploy | kubectl apply --server-side -f -

.PHONY: addon-undeploy
addon-undeploy: download-crds
	$(KUSTOMIZE) build ./deploy | kubectl delete -f -
