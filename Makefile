# Makefile for requestCore project
# Provides convenient commands for version management and development

.PHONY: help version major minor patch version-current version-next version-check version-validate test build clean

# Default target
help:
	@echo "requestCore Development Commands"
	@echo "================================"
	@echo ""
	@echo "Version Management:"
	@echo "  version major      Bump major version (1.0.0 -> 2.0.0)"
	@echo "  version minor      Bump minor version (1.0.0 -> 1.1.0)"
	@echo "  version patch      Bump patch version (1.0.0 -> 1.0.1)"
	@echo "  major              Alias for 'version major'"
	@echo "  minor              Alias for 'version minor'"
	@echo "  patch              Alias for 'version patch'"
	@echo "  version-current    Show current version"
	@echo "  version-next TYPE  Show next version (major|minor|patch)"
	@echo "  version-check      Check if version needs bumping"
	@echo "  version-validate   Validate commit messages"
	@echo ""
	@echo "Development:"
	@echo "  test               Run tests"
	@echo "  build              Build the project"
	@echo "  clean              Clean build artifacts"
	@echo ""
	@echo "Examples:"
	@echo "  make version minor"
	@echo "  make minor"
	@echo "  make patch"
	@echo "  make version-current"

# Version management commands
version:
	@if [ -z "$(filter major minor patch,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make version <major|minor|patch>"; \
		echo "Or use aliases: make major, make minor, make patch"; \
		exit 1; \
	fi

major:
	@./scripts/version.sh bump major

minor:
	@./scripts/version.sh bump minor

patch:
	@./scripts/version.sh bump patch

version-current:
	@./scripts/version.sh current

version-check:
	@./scripts/version.sh check

version-validate:
	@./scripts/version.sh validate

version-next:
	@if [ -z "$(TYPE)" ]; then \
		echo "Usage: make version-next TYPE=<major|minor|patch>"; \
		exit 1; \
	fi
	@./scripts/version.sh next $(TYPE)

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
