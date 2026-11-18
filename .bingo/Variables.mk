# Auto generated binary variables helper managed by https://github.com/bwplotka/bingo v0.9. DO NOT EDIT.
# All tools are designed to be build inside $GOBIN.
BINGO_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
GOPATH ?= $(shell go env GOPATH)
GOBIN  ?= $(firstword $(subst :, ,${GOPATH}))/bin
GO     ?= $(shell which go)

# Below generated variables ensure that every time a tool under each variable is invoked, the correct version
# will be used; reinstalling only if needed.
# For example for golangci-lint variable:
#
# In your main Makefile (for non array binaries):
#
#include .bingo/Variables.mk # Assuming -dir was set to .bingo .
#
#command: $(GOLANGCI_LINT)
#	@echo "Running golangci-lint"
#	@$(GOLANGCI_LINT) <flags/args..>
#
GOLANGCI_LINT := $(GOBIN)/golangci-lint-v2.15.0
$(GOLANGCI_LINT): $(BINGO_DIR)/golangci-lint.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@# NOTE: built from the repo root (not cd'ed into .bingo) so GOTOOLCHAIN
	@# auto-switching applies from the root go.mod; the linter must be built
	@# with a Go toolchain at least as new as the one it lints against.
	@echo "(re)installing $(GOBIN)/golangci-lint-v2.15.0"
	@GOWORK=off $(GO) build -mod=mod -modfile=$(BINGO_DIR)/golangci-lint.mod -o=$(GOBIN)/golangci-lint-v2.15.0 "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"

KUSTOMIZE := $(GOBIN)/kustomize-v5.8.1
$(KUSTOMIZE): $(BINGO_DIR)/kustomize.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/kustomize-v5.8.1"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=kustomize.mod -o=$(GOBIN)/kustomize-v5.8.1 "sigs.k8s.io/kustomize/kustomize/v5"

