# Terraform Provider InfluxDB Makefile
#
# This project uses GoReleaser for official releases following HashiCorp's
# recommended practices for Terraform Registry compatibility.
#
# For releases, use: make goreleaser-release VERSION=v0.1.8
# For local builds: make build or make install
#
.PHONY: build test clean install release help goreleaser-build goreleaser-release

# Variables
BINARY_NAME=terraform-provider-influxdb
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=${VERSION}"
BUILD_DIR=dist

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the provider binary for current platform
	go build ${LDFLAGS} -o ${BINARY_NAME}

test: ## Run tests
	go test ./...

test-verbose: ## Run tests with verbose output
	go test -v ./...

clean: ## Clean build artifacts
	rm -f ${BINARY_NAME}
	rm -f ${BINARY_NAME}_v*
	rm -rf ${BUILD_DIR}
	rm -f *.zip
	rm -f *.tar.gz

install: build ## Install the provider locally for development
	@echo "Installing provider for local development..."
	@mkdir -p ~/.terraform.d/plugins/registry.terraform.io/xing/influxdb/0.1.0/darwin_amd64
	@cp ${BINARY_NAME} ~/.terraform.d/plugins/registry.terraform.io/xing/influxdb/0.1.0/darwin_amd64/

release-notes: ## Add new version to RELEASE_NOTES.md
	@echo "Adding release notes for version ${VERSION}..."
	@if [ ! -f RELEASE_NOTES.md ]; then \
		echo "# Release Notes" > RELEASE_NOTES.md; \
		echo "" >> RELEASE_NOTES.md; \
	fi
	@temp_file=$$(mktemp); \
	echo "## ${VERSION} - $$(date +%Y-%m-%d)" > $$temp_file; \
	echo "" >> $$temp_file; \
	echo "### Features" >> $$temp_file; \
	echo "- Add your features here" >> $$temp_file; \
	echo "" >> $$temp_file; \
	echo "### Bug Fixes" >> $$temp_file; \
	echo "- Add your bug fixes here" >> $$temp_file; \
	echo "" >> $$temp_file; \
	echo "### Changes" >> $$temp_file; \
	echo "- Add your changes here" >> $$temp_file; \
	echo "" >> $$temp_file; \
	cat RELEASE_NOTES.md >> $$temp_file; \
	mv $$temp_file RELEASE_NOTES.md; \
	echo "Release notes added to RELEASE_NOTES.md. Please edit the file to add specific changes."

tag: ## Create and push a git tag
	@if [ -z "${VERSION}" ] || [ "${VERSION}" = "dev" ]; then \
		echo "Error: VERSION must be set to create a tag (e.g., make tag VERSION=v0.1.1)"; \
		exit 1; \
	fi
	@echo "Creating tag ${VERSION}..."
	git tag ${VERSION}
	git push origin ${VERSION}

github-release: ## Create GitHub release (use GoReleaser instead)
	@echo "Warning: github-release is deprecated. Use 'make goreleaser-release' instead."
	@echo "GoReleaser creates better releases with proper Terraform Registry compatibility."

# GoReleaser targets
goreleaser-build: ## Build with GoReleaser (snapshot)
	@echo "Building snapshot with GoReleaser..."
	@goreleaser build --snapshot --clean

goreleaser-release: ## Create release with GoReleaser
	@if [ -z "${VERSION}" ] || [ "${VERSION}" = "dev" ]; then \
		echo "Error: VERSION must be set for release (e.g., make goreleaser-release VERSION=v0.1.8)"; \
		exit 1; \
	fi
	@if [ -z "${GPG_FINGERPRINT}" ]; then \
		echo "Error: GPG_FINGERPRINT must be set (e.g., export GPG_FINGERPRINT=your-key-id)"; \
		exit 1; \
	fi
	@echo "Creating release ${VERSION} with GoReleaser..."
	@GITHUB_TOKEN=$$(gh auth token) goreleaser release --clean

release: goreleaser-release ## Alias for goreleaser-release

dev-release: ## Quick release for current platform only (for development)
	@version=$$(git describe --tags --always --dirty); \
	echo "Creating development release $$version..."; \
	go build ${LDFLAGS} -o ${BINARY_NAME}_v$$version; \
	zip ${BINARY_NAME}_v$$version.zip ${BINARY_NAME}_v$$version; \
	echo "Development binary created: ${BINARY_NAME}_v$$version.zip"

fmt: ## Format Go code
	go fmt ./...

lint: ## Run linter
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, using go vet instead"; \
		go vet ./...; \
	fi

docs: ## Generate documentation
	go generate ./...

check: test lint ## Run tests and linting

# Version management targets
version: ## Show current version
	@echo ${VERSION}

next-patch: ## Show next patch version
	@current=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	echo $$current | awk -F. '{$$NF = $$NF + 1;} 1' | sed 's/ /./g'

next-minor: ## Show next minor version  
	@current=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	echo $$current | awk -F. '{$$(NF-1) = $$(NF-1) + 1; $$NF = 0} 1' | sed 's/ /./g'

next-major: ## Show next major version
	@current=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	echo $$current | awk -F. '{$$1 = substr($$1,1,1) (substr($$1,2) + 1); $$(NF-1) = 0; $$NF = 0} 1' | sed 's/ /./g'