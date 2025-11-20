.PHONY: build test clean install release help

# Variables
BINARY_NAME=terraform-provider-influxdb
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=${VERSION}"
PLATFORMS=darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64
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

build-all: clean ## Build binaries for all platforms
	@echo "Building binaries for all platforms..."
	@mkdir -p ${BUILD_DIR}
	@for platform in ${PLATFORMS}; do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		binary_name=${BINARY_NAME}_${VERSION}_$${os}_$${arch}; \
		if [ "$$os" = "windows" ]; then \
			binary_name=$${binary_name}.exe; \
		fi; \
		echo "Building $$binary_name..."; \
		GOOS=$$os GOARCH=$$arch go build ${LDFLAGS} -o ${BUILD_DIR}/$$binary_name; \
	done

package: build-all ## Package binaries into archives
	@echo "Packaging binaries..."
	@cd ${BUILD_DIR} && for file in ${BINARY_NAME}_${VERSION}_*; do \
		if [[ $$file == *.exe ]]; then \
			zip $${file%.exe}.zip $$file; \
		else \
			tar -czf $${file}.tar.gz $$file; \
		fi; \
	done
	@echo "Creating checksums..."
	@cd ${BUILD_DIR} && shasum -a 256 *.zip *.tar.gz > ${BINARY_NAME}_${VERSION}_SHA256SUMS
	@echo "Signing checksums..."
	@cd ${BUILD_DIR} && gpg --detach-sign --armor ${BINARY_NAME}_${VERSION}_SHA256SUMS
	@cd ${BUILD_DIR} && gpg --detach-sign --output ${BINARY_NAME}_${VERSION}_SHA256SUMS.sig ${BINARY_NAME}_${VERSION}_SHA256SUMS
	@echo "Copying manifest..."
	@cp terraform-registry-manifest.json ${BUILD_DIR}/${BINARY_NAME}_${VERSION}_manifest.json

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

github-release: package ## Create GitHub release with artifacts
	@if [ -z "${VERSION}" ] || [ "${VERSION}" = "dev" ]; then \
		echo "Error: VERSION must be set to create a release (e.g., make github-release VERSION=v0.1.1)"; \
		exit 1; \
	fi
	@echo "Pushing tag ${VERSION}..."
	@git push origin ${VERSION}
	@echo "Creating GitHub release ${VERSION}..."
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "Error: GitHub CLI (gh) is not installed. Please install it first."; \
		exit 1; \
	fi
	@temp_notes=$$(mktemp); \
	awk "/^## ${VERSION}/"',/^## [^${VERSION}]/{if(/^## [^${VERSION}]/) exit; print}' RELEASE_NOTES.md > $$temp_notes; \
	gh release create ${VERSION} \
		${BUILD_DIR}/* \
		--title "Release ${VERSION}" \
		--notes-file $$temp_notes; \
	rm $$temp_notes

release: clean ## Full release process (build, package, tag, GitHub release)
	@if [ -z "${VERSION}" ] || [ "${VERSION}" = "dev" ]; then \
		echo "Error: VERSION must be set for release (e.g., make release VERSION=v0.1.1)"; \
		exit 1; \
	fi
	@echo "Starting full release process for ${VERSION}..."
	@$(MAKE) tag VERSION=${VERSION}
	@$(MAKE) github-release VERSION=${VERSION}
	@echo "Release ${VERSION} completed successfully!"

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