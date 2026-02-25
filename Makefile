# K8sWatch Makefile

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Version information
VERSION ?= 0.1.0
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Image URL to use all building/pushing image targets
IMG ?= k8swatch/agent:$(VERSION)
IMG_AGGREGATOR ?= k8swatch/aggregator:$(VERSION)
IMG_ALERTMANAGER ?= k8swatch/alertmanager:$(VERSION)

# CONTAINER_TOOL defines the container tool to be used for building images.
CONTAINER_TOOL ?= docker

# Path to kind cluster
KIND_CLUSTER_NAME ?= k8swatch-dev

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd paths="./api/v1/..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object paths="./api/v1/..."

.PHONY: generate-proto
generate-proto: ## Generate Go code from protobuf definitions.
	./scripts/generate-proto.sh

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint against code.
	$(GOLANGCI_LINT) run ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -race -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: test-unit
test-unit: ## Run unit tests only.
	go test ./api/... ./internal/... -race -coverprofile=coverage.out -covermode=atomic

.PHONY: tidy
tidy: ## Run go mod tidy.
	go mod tidy

##@ Build

.PHONY: build
build: fmt vet ## Build all binaries.
	go build -o bin/agent -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)" ./cmd/agent
	go build -o bin/aggregator -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)" ./cmd/aggregator
	go build -o bin/alertmanager -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)" ./cmd/alertmanager

.PHONY: build-agent
build-agent: fmt vet ## Build agent binary.
	go build -o bin/agent -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)" ./cmd/agent

.PHONY: build-aggregator
build-aggregator: fmt vet ## Build aggregator binary.
	go build -o bin/aggregator -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)" ./cmd/aggregator

.PHONY: build-alertmanager
build-alertmanager: fmt vet ## Build alertmanager binary.
	go build -o bin/alertmanager -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)" ./cmd/alertmanager

.PHONY: run-agent
run-agent: ## Run agent locally (for development).
	go run ./cmd/agent --kubeconfig=${KUBECONFIG}

.PHONY: run-aggregator
run-aggregator: ## Run aggregator locally (for development).
	go run ./cmd/aggregator --kubeconfig=${KUBECONFIG}

.PHONY: run-alertmanager
run-alertmanager: ## Run alertmanager locally (for development).
	go run ./cmd/alertmanager --kubeconfig=${KUBECONFIG}

##@ Container

.PHONY: docker-build
docker-build: ## Build docker images for all components.
	$(CONTAINER_TOOL) build -t ${IMG} -f deploy/agent/Dockerfile .
	$(CONTAINER_TOOL) build -t ${IMG_AGGREGATOR} -f deploy/aggregator/Dockerfile .
	$(CONTAINER_TOOL) build -t ${IMG_ALERTMANAGER} -f deploy/alertmanager/Dockerfile .

.PHONY: docker-build-agent
docker-build-agent: ## Build docker image for agent.
	$(CONTAINER_TOOL) build -t ${IMG} -f deploy/agent/Dockerfile .

.PHONY: docker-build-aggregator
docker-build-aggregator: ## Build docker image for aggregator.
	$(CONTAINER_TOOL) build -t ${IMG_AGGREGATOR} -f deploy/aggregator/Dockerfile .

.PHONY: docker-build-alertmanager
docker-build-alertmanager: ## Build docker image for alertmanager.
	$(CONTAINER_TOOL) build -t ${IMG_ALERTMANAGER} -f deploy/alertmanager/Dockerfile .

.PHONY: docker-push
docker-push: ## Push docker images to registry.
	$(CONTAINER_TOOL) push ${IMG}
	$(CONTAINER_TOOL) push ${IMG_AGGREGATOR}
	$(CONTAINER_TOOL) push ${IMG_ALERTMANAGER}

##@ Deployment

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy all components to the K8s cluster.
	$(KUSTOMIZE) build deploy/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: ## Undeploy all components from the K8s cluster.
	$(KUSTOMIZE) build deploy/default | $(KUBECTL) delete -f -

.PHONY: install-crds
install-crds: ## Install CRDs only.
	$(KUBECTL) apply -f config/crd/bases/

.PHONY: uninstall-crds
uninstall-crds: ## Uninstall CRDs only.
	$(KUBECTL) delete -f config/crd/bases/ || true

##@ Development Cluster

.PHONY: kind-create
kind-create: kind ## Create a kind cluster for development.
	$(KIND) create cluster --name $(KIND_CLUSTER_NAME) --config=config/kind-config.yaml

.PHONY: kind-delete
kind-delete: kind ## Delete the kind cluster.
	$(KIND) delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: kind-load
kind-load: docker-build-agent docker-build-aggregator docker-build-alertmanager kind ## Load docker images into kind cluster.
	$(KIND) load docker-image ${IMG} --name $(KIND_CLUSTER_NAME)
	$(KIND) load docker-image ${IMG_AGGREGATOR} --name $(KIND_CLUSTER_NAME)
	$(KIND) load docker-image ${IMG_ALERTMANAGER} --name $(KIND_CLUSTER_NAME)

.PHONY: deploy-kind
deploy-kind: kind-load install-crds ## Deploy to kind cluster.
	$(KUSTOMIZE) build deploy/default | $(KUBECTL) apply -f -
	$(KUBECTL) wait --for=condition=ready pod -l app.kubernetes.io/name=k8swatch --timeout=120s

##@ Tools

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,v0.20.1)

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,v5.6.0)

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,v0.27.0)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,v1.64.0)

.PHONY: kubectl
kubectl: $(KUBECTL) ## Download kubectl locally if necessary.
$(KUBECTL): $(LOCALBIN)
	$(call go-install-tool,$(KUBECTL),k8s.io/kubectl,v0.33.0)

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

# Define LocalBIN
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Tool binaries
CONTROLLER_GEN = $(LOCALBIN)/controller-gen
KUSTOMIZE = $(LOCALBIN)/kustomize
KIND = $(LOCALBIN)/kind
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
KUBECTL = $(LOCALBIN)/kubectl

##@ Security

.PHONY: security-scan
security-scan: golangci-lint ## Run security scans.
	$(GOLANGCI_LINT) run --enable gosec ./...

.PHONY: govulncheck
govulncheck: ## Run govulncheck.
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

##@ Verification

.PHONY: verify
verify: fmt vet lint test security-scan ## Run all verification checks.
	@echo "All verification checks passed!"

.PHONY: check-generated
check-generated: ## Verify generated files are up to date.
	@echo "Checking if generated files are up to date..."
	$(MAKE) generate
	$(MAKE) manifests
	@if git diff --exit-code --quiet; then \
		echo "Generated files are up to date."; \
	else \
		echo "Generated files are not up to date. Please run 'make generate' and 'make manifests' and commit the changes."; \
		git diff; \
		exit 1; \
	fi
