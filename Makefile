# include the bingo binary variables. This enables the bingo versions to be
# referenced here as make variables. For example: $(GOLANGCI_LINT)
include .bingo/Variables.mk

# set the default target here, because the include above will automatically set
# it to the first defined target
.DEFAULT_GOAL := default
default: all

VERSION ?= v0.0.1

CRD_DIR := $(shell pwd)/deploy/crds
TEST_CRD_DIR := $(CURDIR)/test/integration/crds
BIN_DIR := $(CURDIR)/bin
CONTAINER_ENGINE ?= $(shell which podman 2>/dev/null || which docker 2>/dev/null)

# REGISTRY_BASE
# defines the container registry and organization for the bundle and operator container images.
REGISTRY_BASE_OPENSHIFT = quay.io/rhobs
REGISTRY_BASE ?= $(REGISTRY_BASE_OPENSHIFT)

# Image URL to use all building/pushing image targets
IMG ?= $(REGISTRY_BASE)/multicluster-observability-addon:$(VERSION)
PLATFORM ?= linux/amd64

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
	@curl https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/refs/tags/v0.83.0-rhobs1/example/prometheus-operator-crd/monitoring.rhobs_prometheusagents.yaml  > $(CRD_DIR)/monitoring.coreos.com_prometheusagents.yaml

$(CRD_DIR)/monitoring.coreos.com_scrapeconfigs.yaml:
	@mkdir -p $(CRD_DIR)
	@curl https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/refs/tags/v0.83.0-rhobs1/example/prometheus-operator-crd/monitoring.rhobs_scrapeconfigs.yaml  > $(CRD_DIR)/monitoring.coreos.com_scrapeconfigs.yaml

$(CRD_DIR)/monitoring.rhobs_prometheusrules.yaml:
	@mkdir -p $(CRD_DIR)
	@curl https://raw.githubusercontent.com/rhobs/obo-prometheus-operator/refs/tags/v0.83.0-rhobs1/example/prometheus-operator-crd/monitoring.rhobs_prometheusrules.yaml  > $(CRD_DIR)/monitoring.rhobs_prometheusrules.yaml

$(CRD_DIR)/core.observatorium.io_observatoria.yaml:
	@mkdir -p $(CRD_DIR)
	@curl -s -L https://raw.githubusercontent.com/stolostron/observatorium-operator/main/manifests/crds/core.observatorium.io_observatoria.yaml > $(CRD_DIR)/core.observatorium.io_observatoria.yaml

# CRDs synchronized from deploy/crds into test/integration/crds for envtest
SHARED_INTEGRATION_CRDS := \
	observability.openshift.io_clusterlogforwarders.yaml \
	opentelemetry.io_opentelemetrycollectors.yaml \
	opentelemetry.io_instrumentations.yaml \
	monitoring.rhobs_prometheusagents.yaml \
	monitoring.rhobs_scrapeconfigs.yaml \
	monitoring.rhobs_prometheusrules.yaml

.PHONY: sync-integration-crds
sync-integration-crds: ## Sync shared operand CRDs from deploy/crds to test/integration/crds
	@mkdir -p $(TEST_CRD_DIR)
	@for crd in $(SHARED_INTEGRATION_CRDS); do \
		cp $(CRD_DIR)/$$crd $(TEST_CRD_DIR)/$$crd; \
	done

.PHONY: download-crds
download-crds: $(CRD_DIR)/observability.openshift.io_clusterlogforwarders.yaml $(CRD_DIR)/opentelemetry.io_opentelemetrycollectors.yaml $(CRD_DIR)/opentelemetry.io_instrumentations.yaml $(CRD_DIR)/monitoring.coreos.com_prometheusagents.yaml $(CRD_DIR)/monitoring.coreos.com_scrapeconfigs.yaml $(CRD_DIR)/monitoring.rhobs_prometheusrules.yaml $(CRD_DIR)/core.observatorium.io_observatoria.yaml
	@$(MAKE) sync-integration-crds

SHELLCHECK ?= $(shell which shellcheck 2>/dev/null)
SHELL_SCRIPTS := $(shell find . -name "*.sh" -not -path "*/.*" -not -path "*/vendor/*")

.PHONY: verify-dockerfile-labels
verify-dockerfile-labels: ## Verify Dockerfile.Konflux RHEL version consistency
	@./hack/verify-dockerfile-labels.sh

.PHONY: lint-scripts
lint-scripts: ## Run shellcheck on repository shell scripts
	@if [ -n "$(SHELLCHECK)" ]; then \
		echo "Running shellcheck on shell scripts..."; \
		$(SHELLCHECK) $(SHELL_SCRIPTS); \
	elif [ -n "$(CONTAINER_ENGINE)" ]; then \
		echo "Running shellcheck via $(CONTAINER_ENGINE)..."; \
		$(CONTAINER_ENGINE) run --rm -v "$(CURDIR):/mnt:ro" -w /mnt docker.io/koalaman/shellcheck:stable $(SHELL_SCRIPTS); \
	else \
		echo "Warning: shellcheck not found and no container engine available. Skipping shell lint."; \
	fi

.PHONY: lint
lint: $(GOLANGCI_LINT) verify-dockerfile-labels lint-scripts ## Run all linters (golangci-lint, dockerfile, shellcheck)
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run --timeout=5m ./...

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Attempt to automatically fix lint issues in source code.
	$(GOLANGCI_LINT) run --fix --timeout=5m ./...

.PHONY: fmt
fmt: lint-fix

.PHONY: test
test:
	go test ./internal/...

.PHONY: bench
bench: ## Run Go performance benchmarks with memory allocation profiling
	go test -run=^# -bench=. -benchmem ./internal/...
ENVTEST_K8S_VERSION ?= 1.35.0
SETUP_ENVTEST ?= $(BIN_DIR)/setup-envtest

$(SETUP_ENVTEST): prepare-bin
	@if [ ! -f $(SETUP_ENVTEST) ]; then \
		echo "Installing setup-envtest to $(BIN_DIR)..."; \
		GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.23; \
	fi

.PHONY: envtest-bins
envtest-bins: $(SETUP_ENVTEST) ## Download envtest kubernetes binaries
	@$(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(BIN_DIR) > /dev/null

.PHONY: test-integration
test-integration: envtest-bins ## Run controller integration tests using envtest
	KUBEBUILDER_ASSETS="$$($(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(BIN_DIR) -p path)" \
	go test -v ./test/integration/...

.PHONY: prepare-bin
prepare-bin:
	@mkdir -p $(BIN_DIR)

.PHONY: addon
addon: deps fmt ## Build addon binary
	go build -o bin/multicluster-observability-addon main.go

.PHONY: oci-build
oci-build: ## Build the image
	$(CONTAINER_ENGINE) build --platform $(PLATFORM) -t ${IMG} .

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

.PHONY: test-e2e-setup
test-e2e-setup: ## Setup KinD cluster and dependencies for E2E tests
	./test/scripts/setup-kind.sh

.PHONY: test-e2e-run
test-e2e-run: ## Run E2E tests against running KinD cluster
	KUBECONFIG=/tmp/mcoa-e2e.kubeconfig go test -v -tags=e2e -count=1 -timeout=15m ./test/e2e/...

.PHONY: test-e2e-clean
test-e2e-clean: ## Teardown KinD E2E cluster
	./test/scripts/cleanup-kind.sh

.PHONY: test-e2e
test-e2e: test-e2e-setup test-e2e-run ## Run complete E2E test lifecycle
