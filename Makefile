# Makefile for requestCore project
# Provides convenient commands for version management and development

.PHONY: help version-current version-next version-bump version-release version-check version-validate test build clean

# Default target
help:
	@echo "requestCore Development Commands"
	@echo "================================"
	@echo ""
	@echo "Version Management:"
	@echo "  version-current     Show current version"
	@echo "  version-next TYPE   Show next version (major|minor|patch)"
	@echo "  version-bump TYPE   Bump version and create tag"
	@echo "  version-release TYPE Create release with version bump"
	@echo "  version-check       Check if version needs bumping"
	@echo "  version-validate    Validate commit messages"
	@echo ""
	@echo "Development:"
	@echo "  test               Run tests"
	@echo "  build              Build the project"
	@echo "  clean              Clean build artifacts"
	@echo ""
	@echo "Examples:"
	@echo "  make version-current"
	@echo "  make version-next minor"
	@echo "  make version-bump patch"
	@echo "  make version-release major"

# Version management commands
version-current:
	@./scripts/version.sh current

version-next:
	@if [ -z "$(TYPE)" ]; then \
		echo "Usage: make version-next TYPE=<major|minor|patch>"; \
		exit 1; \
	fi
	@./scripts/version.sh next $(TYPE)

version-bump:
	@if [ -z "$(TYPE)" ]; then \
		echo "Usage: make version-bump TYPE=<major|minor|patch>"; \
		exit 1; \
	fi
	@./scripts/version.sh bump $(TYPE)

version-release:
	@if [ -z "$(TYPE)" ]; then \
		echo "Usage: make version-release TYPE=<major|minor|patch>"; \
		exit 1; \
	fi
	@./scripts/version.sh release $(TYPE)

version-check:
	@./scripts/version.sh check

version-validate:
	@./scripts/version.sh validate

# Development commands
test:
	@echo "Running tests..."
	@go test ./...

build:
	@echo "Building requestCore..."
	@go build -o bin/requestCore .

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@go clean

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@go mod download

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@go vet ./...

# Run all checks
check: fmt lint test
	@echo "All checks passed!"

# Quick development workflow
dev: check build
	@echo "Development build complete!"
