BINARY   := skillman
MODULE   := github.com/melvicsosa/skillman
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w \
	-X $(MODULE)/internal/app/version.Version=$(VERSION) \
	-X $(MODULE)/internal/app/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/app/version.Date=$(DATE)

.PHONY: build test vet lint web run clean

build: ## Build the binary into ./dist
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o dist/$(BINARY) ./cmd/$(BINARY)

test: ## Run Go tests
	go test ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint if installed, otherwise go vet
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else echo "golangci-lint not found, running go vet"; go vet ./...; fi

web: ## Install and build the frontend into web/dist
	cd web && pnpm install --frozen-lockfile && pnpm build

run: ## Start the local server
	go run ./cmd/$(BINARY) serve

clean:
	rm -rf dist
