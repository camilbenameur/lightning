# Lightning developer tasks. `make` lists them. Works on Linux, macOS and WSL.
# Needs Go, Node 24 and Docker (with Compose 2.24+).

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)
LANGUAGES ?= python javascript typescript go rust c cpp java ruby php bash
BASE_URL ?= http://localhost:8088
GOLANGCI_LINT ?= golangci-lint
COMPOSE_LOCAL := docker compose -f compose.yaml -f compose.local.yaml

.PHONY: help dev-api dev-web test test-integration lint fmt build images runtimes runtimes-test up down logs smoke

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  \033[36m%-17s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev-api: ## Run the API on :8080 against the local Docker daemon
	LIGHTNING_LOG_FORMAT=text go run -ldflags "$(LDFLAGS)" ./cmd/lightning

dev-web: web/node_modules ## Run the Vite dev server (proxies /api to :8080)
	npm --prefix web run dev

web/node_modules: web/package-lock.json
	npm --prefix web ci
	@touch $@

test: web/node_modules ## Run the Go (race detector) and web unit tests
	go test -race ./...
	npm --prefix web test

test-integration: ## Run the Go integration tests (needs `make runtimes LANGUAGES="python javascript"`)
	go test -race -tags integration ./...

lint: web/node_modules ## Run go vet, golangci-lint, ESLint, Prettier and tsc
	go vet ./...
	$(GOLANGCI_LINT) run
	npm --prefix web run lint
	npm --prefix web run format:check
	npm --prefix web run typecheck

fmt: web/node_modules ## Format the Go and web sources
	gofmt -w cmd internal
	npm --prefix web run format

build: web/node_modules ## Build bin/lightning and the web bundle (web/dist)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/lightning ./cmd/lightning
	npm --prefix web run build

images: ## Build the lightning-api:dev and lightning-web:dev images
	$(COMPOSE_LOCAL) build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT)

runtimes: ## Build the sandbox runtime images (LANGUAGES="python go" for a subset)
	bash runtimes/build.sh $(LANGUAGES)

runtimes-test: ## Test the runtime images under the production sandbox flags
	bash runtimes/test.sh $(LANGUAGES)

up: images ## Start the stack from source on http://localhost:8088
	$(COMPOSE_LOCAL) up --detach --wait

down: ## Stop the local stack
	$(COMPOSE_LOCAL) down --remove-orphans

logs: ## Follow the local stack logs
	$(COMPOSE_LOCAL) logs --follow

smoke: ## Smoke-test a running stack (BASE_URL, default http://localhost:8088)
	bash scripts/smoke.sh $(BASE_URL)
