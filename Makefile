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
LINTCMD=golangci-lint-v2

# -----------------------------------------------------------------------------
# Build Variables
# -----------------------------------------------------------------------------
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "unknown")

# Build flags for injecting version info
LDFLAGS := "-X 'main.Version=$(VERSION)' -X 'main.CommitHash=$(GIT_COMMIT)'"

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
	$(GOCMD) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GOCMD) install github.com/securego/gosec/v2/cmd/gosec@latest

# -----------------------------------------------------------------------------
# Testing
# -----------------------------------------------------------------------------

test: test-up
	gotestsum --format pkgname -- -race -parallel=8 ./...

test-nocache: test-up
	gotestsum --format pkgname -- -race -count=1 -parallel=8 ./...

# Start test dependencies (Neo4j)
test-up:
	@echo "Starting test dependencies..."
	@docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for Neo4j to be ready..."
	@docker-compose -f docker-compose.test.yml exec -T neo4j-test wget -q --spider --tries=30 --waitretry=2 http://localhost:7474 || true
	@echo "Test dependencies ready"

# Stop test dependencies
test-down:
	@echo "Stopping test dependencies..."
	@docker-compose -f docker-compose.test.yml down

# Clean test dependencies (including volumes)
test-clean:
	@echo "Cleaning test dependencies..."
	@docker-compose -f docker-compose.test.yml down -v

# Show test logs
test-logs:
	@docker-compose -f docker-compose.test.yml logs -f

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

test-coverage:
	@echo "Running tests with coverage..."
	$(GOCMD) test -race -coverprofile=coverage.out -covermode=atomic ./...
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
	$(GOCMD) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GOCMD) install github.com/securego/gosec/v2/cmd/gosec@latest

ci-test: test-up test test-integration test-down

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