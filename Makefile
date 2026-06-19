# Decentralized Messenger — common developer tasks.

BINARY      := messenger
BIN_DIR     := bin
PKG         := ./...
DOCKER_IMAGE ?= decentralized-messenger:latest

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the messenger binary into bin/
	go build -trimpath -o $(BIN_DIR)/$(BINARY) ./cmd/messenger

.PHONY: run
run: ## Run the HTTP node (in-memory adapters)
	go run ./cmd/messenger

.PHONY: demo
demo: ## Run the in-process demonstration
	go run ./cmd/messenger -demo

.PHONY: test
test: ## Run tests with the race detector
	go test -race -count=1 $(PKG)

.PHONY: vet
vet: ## Run go vet
	go vet $(PKG)

.PHONY: fmt
fmt: ## Format all Go source
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if any file is not gofmt-formatted
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Not gofmt-formatted:"; echo "$$unformatted"; exit 1; \
	fi

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum
	go mod tidy

.PHONY: check
check: fmt-check vet build test ## Run the full CI suite locally

.PHONY: docker
docker: ## Build the Docker image
	docker build -f docker/Dockerfile -t $(DOCKER_IMAGE) .

.PHONY: compose-up
compose-up: ## Start the full stack (ScyllaDB, Redis, RabbitMQ, node)
	docker compose up --build

.PHONY: compose-down
compose-down: ## Stop the stack and remove volumes
	docker compose down -v

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
