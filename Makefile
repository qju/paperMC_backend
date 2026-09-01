# ==============================================================================
# Lodestone - Makefile
# ==============================================================================

.PHONY: all build build-all build-arm64 build-amd64 build-arm frontend test dev deploy clean help

# Default target
all: build

## help: Display available make targets
help:
	@echo "Lodestone Build & Automation Menu"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build React frontend and native Go binary"
	@echo "  build-arm64   Cross-compile for Linux ARM64 (Raspberry Pi 4/5, Oracle Ampere, etc.)"
	@echo "  build-amd64   Cross-compile for Linux AMD64 (Standard x86_64 VPS / Servers)"
	@echo "  build-arm     Cross-compile for Linux ARMv7 (32-bit ARM)"
	@echo "  build-all     Build binaries for all architectures into bin/"
	@echo "  frontend      Rebuild only the React frontend bundle into web/dist/"
	@echo "  test          Run all Go automated tests with statement coverage"
	@echo "  deploy        Execute automated remote deployment script (./scripts/deploy.sh)"
	@echo "  dev           Start the local development server and UI"
	@echo "  clean         Clean up compiled binaries and build artifacts"

## frontend: Build the React frontend SPA
frontend:
	@echo "⚛️  Building React frontend..."
	@cd web && npm install && npm run build

## build: Build frontend and native binary
build: frontend
	@echo "🔨 Building native binary..."
	@mkdir -p bin
	@CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/lodestone ./cmd/server/main.go
	@echo "✓ Binary created at bin/lodestone"

## build-arm64: Cross-compile for Linux ARM64
build-arm64:
	@./scripts/build.sh arm64

## build-amd64: Cross-compile for Linux AMD64
build-amd64:
	@./scripts/build.sh amd64

## build-arm: Cross-compile for Linux ARM 32-bit
build-arm:
	@./scripts/build.sh arm

## build-all: Cross-compile for all supported architectures
build-all:
	@./scripts/build.sh all

## test: Run backend test suite
test:
	@echo "🧪 Running Go test suite..."
	@go test -v -cover ./...

## dev: Launch development environment
dev:
	@./dev.sh

## deploy: Run deployment script (supports TARGET_HOST, TARGET_DIR, etc.)
deploy:
	@./scripts/deploy.sh $(ARGS)

## clean: Remove build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf bin/ dist/ web/dist/
	@echo "✓ Cleanup complete"
