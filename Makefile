BINARY_NAME ?= openvox-code
GOBIN ?= $(shell go env GOPATH)/bin
IMAGE_REGISTRY ?= ghcr.io/slauger
IMAGE_NAME ?= $(IMAGE_REGISTRY)/openvox-code
IMAGE_TAG ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
CONTAINER_TOOL ?= $(shell which podman 2>/dev/null || which docker 2>/dev/null)

.PHONY: all
all: build

##@ Development

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: vulncheck
vulncheck: ## Run govulncheck
	govulncheck ./...

.PHONY: test
test: ## Run tests with coverage
	go test ./... -coverprofile cover.out -race
	@echo "Coverage report: cover.out"

.PHONY: test-short
test-short: ## Run short tests only
	go test -short ./... -race

##@ Build

.PHONY: build
build: fmt vet ## Build the binary
	go build -trimpath -ldflags "-X main.version=$(IMAGE_TAG) -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/$(BINARY_NAME) ./cmd/openvox-code

.PHONY: install
install: build ## Install binary to GOPATH/bin
	cp bin/$(BINARY_NAME) $(GOBIN)/$(BINARY_NAME)

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/ dist/ cover.out

##@ Container

.PHONY: container-build
container-build: ## Build container image
	$(CONTAINER_TOOL) build -f Containerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .

.PHONY: container-push
container-push: ## Push container image
	$(CONTAINER_TOOL) push $(IMAGE_NAME):$(IMAGE_TAG)

##@ CI

.PHONY: ci
ci: lint vet test vulncheck ## Run all CI checks
	@echo "All CI checks passed."

.PHONY: check-tidy
check-tidy: ## Check go.mod is tidy
	go mod tidy
	git diff --exit-code go.mod go.sum

##@ Help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
