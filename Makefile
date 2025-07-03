-include .env
# Makefile for gograph Go Project

# -----------------------------------------------------------------------------
# Go Parameters & Setup
# -----------------------------------------------------------------------------
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt -s -w
BINARY_NAME=gograph
BINARY_DIR=bin
SRC_DIRS=./...
LINTCMD=golangci-lint

# -----------------------------------------------------------------------------
# Build Variables
# -----------------------------------------------------------------------------
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "unknown")

# Build flags for injecting version info
LDFLAGS := -X 'main.Version=$(VERSION)' -X 'main.CommitHash=$(GIT_COMMIT)'

.PHONY: all test lint fmt clean build deps help
.PHONY: tidy run-neo4j stop-neo4j clean-neo4j
.PHONY: test-up test-down test-clean test-logs

# -----------------------------------------------------------------------------
# Main Targets
# -----------------------------------------------------------------------------
all: test lint fmt

clean:
	rm -rf $(BINARY_DIR)/
	$(GOCMD) clean

build:
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME) cmd/gograph/main.go
	chmod +x $(BINARY_DIR)/$(BINARY_NAME)

# -----------------------------------------------------------------------------
# Code Quality & Formatting
# -----------------------------------------------------------------------------
lint:
	$(LINTCMD) run --fix --allow-parallel-runners
	@echo "Linting completed successfully"

fmt:
	@echo "Formatting code..."
	$(LINTCMD) fmt
	@echo "Formatting completed successfully"

# -----------------------------------------------------------------------------
# Development & Dependencies
# -----------------------------------------------------------------------------

tidy:
	@echo "Tidying modules..."
	$(GOCMD) mod tidy

deps: 
	$(GOCMD) install gotest.tools/gotestsum@latest
	$(GOCMD) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GOCMD) install github.com/securego/gosec/v2/cmd/gosec@latest

# -----------------------------------------------------------------------------
# Testing
# -----------------------------------------------------------------------------

test: test-up
	gotestsum --format pkgname -- -race -parallel=4 ./...

test-nocache: test-up
	gotestsum --format pkgname -- -race -count=1 -parallel=4 ./...

# Start test dependencies (Neo4j)
test-up:
	@echo "Starting test dependencies..."
ifeq ($(CI),true)
	@echo "CI environment detected - Neo4j service should already be running"
	@echo "Checking Neo4j connectivity..."
	@for i in $$(seq 1 15); do \
		if curl -f http://localhost:7474 >/dev/null 2>&1 || wget -q --spider http://localhost:7474 >/dev/null 2>&1; then \
			echo "Neo4j is ready"; \
			break; \
		else \
			echo "Waiting for Neo4j... (attempt $$i/15)"; \
			sleep 2; \
		fi; \
	done
else
	@command -v docker-compose >/dev/null 2>&1 || { echo >&2 "docker-compose is required but not installed. Aborting."; exit 1; }
	@docker-compose -f docker-compose.yml up -d
	@echo "Waiting for Neo4j to be ready..."
	@docker-compose -f docker-compose.yml exec -T neo4j-test wget -q --spider --tries=30 --waitretry=2 http://localhost:7474 || true
endif
	@echo "Test dependencies ready"

# Stop test dependencies
test-down:
	@echo "Stopping test dependencies..."
ifeq ($(CI),true)
	@echo "CI environment detected - skipping docker-compose down"
else
	@docker-compose -f docker-compose.yml down
endif

# Clean test dependencies (including volumes)
test-clean:
	@echo "Cleaning test dependencies..."
ifeq ($(CI),true)
	@echo "CI environment detected - skipping docker-compose cleanup"
else
	@docker-compose -f docker-compose.yml down -v
endif

# Show test logs
test-logs:
ifeq ($(CI),true)
	@echo "CI environment detected - use GitHub Actions logs"
else
	@docker-compose -f docker-compose.yml logs -f
endif

# -----------------------------------------------------------------------------
# Neo4j Management
# -----------------------------------------------------------------------------
run-neo4j:
	@echo "Starting Neo4j..."
	@docker run -d \
		--name gograph-neo4j \
		-p 7474:7474 -p 7687:7687 \
		-e NEO4J_AUTH=neo4j/password \
		-e NEO4J_PLUGINS='["apoc","graph-data-science"]' \
		neo4j:5-community
	@echo "Neo4j started at http://localhost:7474"

stop-neo4j:
	@echo "Stopping Neo4j..."
	@docker stop gograph-neo4j || true
	@docker rm gograph-neo4j || true
	@echo "Neo4j stopped"

clean-neo4j:
	@echo "Cleaning Neo4j data..."
	@docker stop gograph-neo4j || true
	@docker rm -v gograph-neo4j || true
	@echo "Neo4j data cleaned"

# -----------------------------------------------------------------------------
# Development
# -----------------------------------------------------------------------------
dev: run-neo4j
	@echo "Starting development environment..."
	@echo "Neo4j running at http://localhost:7474"
	@echo "Run 'make build' to build the binary"
	@echo "Run 'make test' to run tests"

# -----------------------------------------------------------------------------
# Migration Commands
# -----------------------------------------------------------------------------
migrate-up:
	@echo "Running database migrations..."
	@docker exec gograph-neo4j cypher-shell -u neo4j -p password -f /dev/stdin < migrations/create_indexes.cypher || true

migrate-down:
	@echo "Rolling back database migrations..."
	@docker exec gograph-neo4j cypher-shell -u neo4j -p password "DROP CONSTRAINT IF EXISTS unique_project_id; DROP INDEX IF EXISTS node_project_id; DROP INDEX IF EXISTS rel_project_id;" || true

migrate-status:
	@echo "Checking migration status..."
	@docker exec gograph-neo4j cypher-shell -u neo4j -p password "SHOW INDEXES; SHOW CONSTRAINTS;" || true

reset-db: stop-neo4j clean-neo4j run-neo4j
	@echo "Database reset complete"

# -----------------------------------------------------------------------------
# Code Quality & Coverage
# -----------------------------------------------------------------------------
test-integration: test-up
	gotestsum --format pkgname -- -race -tags=integration ./test/integration/...

test-coverage: test-up
	@echo "Running tests with coverage..."
ifeq ($(CI),true)
	@echo "Building binary in CI environment..."
	@mkdir -p $(BINARY_DIR)
	@$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME) cmd/gograph/main.go
	@chmod +x $(BINARY_DIR)/$(BINARY_NAME)
endif
	gotestsum --format pkgname -- -race -parallel=4 -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-benchmarks:
	@echo "Running benchmarks..."
	$(GOCMD) test -bench=. -benchmem ./...

# -----------------------------------------------------------------------------
# Security & Analysis
# -----------------------------------------------------------------------------
security-scan:
	@echo "Running security scan..."
	gosec ./... || echo "Security issues found - review required"

vulnerability-check:
	@echo "Checking for known vulnerabilities..."
	govulncheck ./...

# -----------------------------------------------------------------------------
# Release & Deployment
# -----------------------------------------------------------------------------
install:
	$(GOCMD) install -ldflags "$(LDFLAGS)" ./cmd/gograph

release-dry:
	@echo "Dry run release build..."
	goreleaser release --snapshot --rm-dist

# -----------------------------------------------------------------------------
# CI/CD Targets
# -----------------------------------------------------------------------------
ci-deps:
	@echo "Installing CI dependencies..."
	$(GOCMD) install gotest.tools/gotestsum@latest
	$(GOCMD) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GOCMD) install github.com/securego/gosec/v2/cmd/gosec@latest

# CI-specific test target that handles Neo4j service properly
ci-test: test-up
	@echo "Building binary for tests..."
	@mkdir -p $(BINARY_DIR)
	@$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(BINARY_NAME) cmd/gograph/main.go
	@chmod +x $(BINARY_DIR)/$(BINARY_NAME)
	gotestsum --format pkgname -- -race -parallel=4 ./...

ci-lint: lint

ci-all: ci-deps ci-lint ci-test

# -----------------------------------------------------------------------------
# Development Tools
# -----------------------------------------------------------------------------
setup-hooks:
	@echo "Setting up pre-commit hooks..."
	@if [ ! -f .git/hooks/pre-commit ]; then \
		cp scripts/pre-commit .git/hooks/pre-commit; \
		chmod +x .git/hooks/pre-commit; \
		echo "Pre-commit hook installed"; \
	else \
		echo "Pre-commit hook already exists"; \
	fi

generate-docs:
	@echo "Generating documentation..."
	$(GOCMD) doc -all ./... > docs/API.md

# -----------------------------------------------------------------------------
# Docker Operations
# -----------------------------------------------------------------------------
docker-build:
	@echo "Building Docker image..."
	docker build -t gograph:latest .

docker-run: docker-build
	@echo "Running Docker container..."
	docker run -it --rm gograph:latest

# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------
help:
	@echo "Available targets:"
	@echo "  all            - Run tests, linting, and formatting"
	@echo "  build          - Build the binary"
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run all tests (starts test dependencies)"
	@echo "  test-up        - Start test dependencies (Neo4j)"
	@echo "  test-down      - Stop test dependencies"
	@echo "  test-clean     - Clean test dependencies and data"
	@echo "  test-logs      - Show test dependency logs"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-integration - Run integration tests"
	@echo "  lint           - Run linter with fixes"
	@echo "  fmt            - Format code"
	@echo "  deps           - Install dependencies"
	@echo "  tidy           - Tidy go modules"
	@echo "  run-neo4j      - Start Neo4j container"
	@echo "  stop-neo4j     - Stop Neo4j container"
	@echo "  dev            - Start development environment"
	@echo "  migrate-up     - Run database migrations"
	@echo "  migrate-down   - Rollback migrations"
	@echo "  reset-db       - Reset database completely"
	@echo "  security-scan  - Run security analysis"
	@echo "  ci-all         - Run full CI pipeline"
	@echo "  setup-hooks    - Install git hooks"
	@echo "  install        - Install binary to GOPATH"
	@echo "  help           - Show this help"