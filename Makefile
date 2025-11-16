.PHONY: build test clean install version tag build-all build-linux build-macos build-windows build-release release

# Get version from git tags, or use 0.1.0 if no tags exist
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "0.1.5")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
LDFLAGS_RELEASE := -s -w -buildid= $(LDFLAGS)
LDFLAGS_RACE := -race $(LDFLAGS)
LDFLAGS_TRIM := -trimpath $(LDFLAGS)
LDFLAGS_OPTIMIZED := -trimpath -ldflags "-s -w" $(LDFLAGS)
BINARY_NAME := chatty
BUILD_DIR := dist

# Optimization flags for development builds
GOFLAGS := -trimpath

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/chatty

# Build with race condition detection for testing
build-race:
	go build -race -ldflags "$(LDFLAGS_RACE)" -o $(BINARY_NAME) ./cmd/chatty

# Build with all optimizations enabled
build-optimized:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS_OPTIMIZED)" -o $(BINARY_NAME) ./cmd/chatty

# Build for all platforms
build-all: build-linux build-macos build-windows
	@echo "All builds completed in $(BUILD_DIR)/"

# Linux builds
build-linux: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/chatty
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

build-linux-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/chatty
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"

# macOS builds
build-macos: build-macos-arm64

build-macos-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64 ./cmd/chatty
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64"

# Windows builds
build-windows: build-windows-amd64 build-windows-arm64

build-windows-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/chatty
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

build-windows-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/chatty
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe"

# Release builds (stripped binaries for smaller size)
build-release: build-release-linux build-release-macos build-release-windows
	@echo "All release builds completed in $(BUILD_DIR)/"
	@echo "Binary sizes:"
	@ls -lh $(BUILD_DIR)

build-release-linux: build-release-linux-amd64 build-release-linux-arm64

build-release-linux-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS_RELEASE)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/chatty
	@echo "Built (stripped): $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

build-release-linux-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS_RELEASE)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/chatty
	@echo "Built (stripped): $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"

build-release-macos: build-release-macos-arm64

build-release-macos-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS_RELEASE)" -o $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64 ./cmd/chatty
	@echo "Built (stripped): $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64"

build-release-windows: build-release-windows-amd64 build-release-windows-arm64

build-release-windows-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS_RELEASE)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/chatty
	@echo "Built (stripped): $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

build-release-windows-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS_RELEASE)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/chatty
	@echo "Built (stripped): $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe"

test:
	go test -race -coverprofile=coverage.out ./...

# Run tests with race detection for thorough testing
test-race:
	go test -race -coverprofile=coverage-race.out ./...

# Run tests with memory profiling
test-profile:
	go test -memprofile=mem.prof -cpuprofile=cpu.prof ./...

clean:
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/chatty

run: build
	./chatty

version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"

release: test compress-release
	@echo "=== Release Checklist ==="
	@echo ""
	@echo "Current version: $(VERSION)"
	@echo "Current commit:  $(COMMIT)"
	@echo ""
	@echo "Pre-release checks:"
	@echo "  ✓ Tests passed"
	@echo "  ✓ Binaries compressed with upx"
	@echo ""
	@read -p "Have you updated CHANGELOG/docs? (y/n): " updated; \
	if [ "$$updated" != "y" ]; then \
		echo "Please update documentation before releasing"; \
		exit 1; \
	fi; \
	echo ""; \
	read -p "Enter new version (e.g., v0.3.0): " new_version; \
	if [ -z "$$new_version" ]; then \
		echo "Error: Version cannot be empty"; \
		exit 1; \
	fi; \
	if ! echo "$$new_version" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "Error: Version must be in format v0.0.0"; \
		exit 1; \
	fi; \
	if git rev-parse "$$new_version" >/dev/null 2>&1; then \
		echo "Error: Tag $$new_version already exists"; \
		exit 1; \
	fi; \
	git tag -a "$$new_version" -m "Release $$new_version"; \
	echo ""; \
	echo "✓ Tagged $$new_version"; \
	echo ""; \
	echo "Next steps:"; \
	echo "  1. Push the tag:    git push origin $$new_version"; \
	echo "  2. GitHub Actions will automatically build and create the release"; \
	echo "  3. View releases:   https://github.com/ZaguanLabs/chatty/releases"

compress-release: build-release
	@if command -v upx >/dev/null; then \
		echo "Compressing release binaries with upx..."; \
		upx --best --lzma $(BUILD_DIR)/$(BINARY_NAME)-*; \
	else \
		echo "Warning: upx not found, skipping binary compression"; \
	fi

tag:
	@echo "Current version: $(VERSION)"
	@echo ""
	@read -p "Enter new version (e.g., v0.3.0): " new_version; \
	if [ -z "$$new_version" ]; then \
		echo "Error: Version cannot be empty"; \
		exit 1; \
	fi; \
	if ! echo "$$new_version" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "Error: Version must be in format v0.0.0"; \
		exit 1; \
	fi; \
	if git rev-parse "$$new_version" >/dev/null 2>&1; then \
		echo "Error: Tag $$new_version already exists"; \
		exit 1; \
	fi; \
	git tag -a "$$new_version" -m "Release $$new_version"; \
	echo ""; \
	echo "✓ Tagged $$new_version"; \
	echo ""; \
	echo "Next steps:"; \
	echo "  1. Push the tag:    git push origin $$new_version"; \
	echo "  2. GitHub Actions will automatically build and create the release"; \
	echo "  3. View releases:   https://github.com/ZaguanLabs/chatty/releases"
