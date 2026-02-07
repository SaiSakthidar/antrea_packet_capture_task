# Makefile for Packet Capture Controller

# Variables
BINARY_NAME=packet-capture-controller
DOCKER_IMAGE=packet-capture-controller:latest
KIND_CLUSTER_NAME=packet-capture-test

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

.PHONY: help
help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: deps
deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

.PHONY: build
build: deps ## Build the controller binary
	$(GOBUILD) -o bin/$(BINARY_NAME) -v ./cmd/controller/

.PHONY: test
test: ## Run unit tests
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: test-coverage
test-coverage: test ## Run tests and generate coverage report
	$(GOCMD) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: clean
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.txt coverage.html

.PHONY: fmt
fmt: ## Format Go code
	$(GOCMD) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GOCMD) vet ./...

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint installed)
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

.PHONY: docker-build
docker-build: ## Build Docker image
	sudo docker build -t $(DOCKER_IMAGE) .

.PHONY: kind-load
kind-load: docker-build ## Load Docker image into Kind cluster
	sudo kind load docker-image $(DOCKER_IMAGE) --name $(KIND_CLUSTER_NAME)

.PHONY: kind-create
kind-create: ## Create Kind cluster
	sudo kind create cluster --config kind-config.yaml --name $(KIND_CLUSTER_NAME)

.PHONY: kind-delete
kind-delete: ## Delete Kind cluster
	sudo kind delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: deploy
deploy: ## Deploy controller to Kubernetes
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/daemonset.yaml

.PHONY: undeploy
undeploy: ## Remove controller from Kubernetes
	kubectl delete -f deploy/daemonset.yaml --ignore-not-found=true
	kubectl delete -f deploy/rbac.yaml --ignore-not-found=true

.PHONY: deploy-test-pod
deploy-test-pod: ## Deploy test pod
	kubectl apply -f deploy/test-pod.yaml

.PHONY: logs
logs: ## Show controller logs
	kubectl logs -l app=packet-capture-controller -n default --tail=100 -f

.PHONY: all
all: clean deps fmt vet test build ## Run all checks and build
