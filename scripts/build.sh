#!/usr/bin/env bash
set -e

# ==============================================================================
# Lodestone Multi-Architecture Build Script
# ==============================================================================

BOLD='\033[1m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
BUILD_FRONTEND=true
TARGET_ARCH="arm64"
TARGET_OS="linux"

usage() {
    echo -e "${BOLD}Usage:${NC} $0 [OPTIONS] [ARCH]"
    echo ""
    echo -e "${BOLD}Architectures:${NC}"
    echo "  arm64        Linux ARM64 / AArch64 (Raspberry Pi 4/5, Oracle Ampere, AWS Graviton) [default]"
    echo "  amd64        Linux x86_64 (Standard VPS, Intel/AMD dedicated servers)"
    echo "  arm          Linux ARMv7 (32-bit ARM devices)"
    echo "  darwin-arm64 macOS Apple Silicon (M1/M2/M3/M4)"
    echo "  darwin-amd64 macOS Intel"
    echo "  all          Build all standard target binaries"
    echo ""
    echo -e "${BOLD}Options:${NC}"
    echo "  --skip-frontend, -s   Skip rebuilding the React frontend bundle"
    echo "  --help, -h            Show this help message"
    exit 0
}

# Parse Arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --skip-frontend|-s)
            BUILD_FRONTEND=false
            shift
            ;;
        --help|-h)
            usage
            ;;
        arm64|amd64|arm|darwin-arm64|darwin-amd64|all)
            TARGET_ARCH="$1"
            shift
            ;;
        *)
            echo -e "${RED}Unknown argument: $1${NC}"
            usage
            ;;
    esac
done

mkdir -p "${BIN_DIR}"

# 1. Build Frontend Bundle
if [ "$BUILD_FRONTEND" = true ]; then
    echo -e "${CYAN}⚛️  Building React Frontend bundle...${NC}"
    cd "${ROOT_DIR}/web"
    if [ ! -d "node_modules" ]; then
        echo -e "${YELLOW}Installing frontend dependencies...${NC}"
        npm install
    fi
    npm run build
    cd "${ROOT_DIR}"
    echo -e "${GREEN}✓ Frontend built successfully into web/dist/${NC}"
else
    echo -e "${YELLOW}⚡ Skipping frontend build step (--skip-frontend)${NC}"
fi

# Build function
build_binary() {
    local os="$1"
    local arch="$2"
    local out_name="lodestone-${os}-${arch}"
    if [ "$os" = "windows" ]; then
        out_name="${out_name}.exe"
    fi
    local out_path="${BIN_DIR}/${out_name}"

    echo -e "${CYAN}🐧 Compiling static binary for ${BOLD}${os}/${arch}${NC} -> ${out_name}..."
    CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build -trimpath -ldflags="-s -w" -o "${out_path}" "${ROOT_DIR}/cmd/server/main.go"
    
    local size=$(ls -lh "${out_path}" | awk '{print $5}')
    echo -e "${GREEN}✓ Successfully built:${NC} ${BOLD}bin/${out_name}${NC} (${size})"
}

echo ""
echo -e "${BOLD}🚀 Compiling Lodestone Go Binaries...${NC}"

case "$TARGET_ARCH" in
    all)
        build_binary "linux" "arm64"
        build_binary "linux" "amd64"
        build_binary "linux" "arm"
        build_binary "darwin" "arm64"
        build_binary "darwin" "amd64"
        ;;
    darwin-arm64)
        build_binary "darwin" "arm64"
        ;;
    darwin-amd64)
        build_binary "darwin" "amd64"
        ;;
    arm64)
        build_binary "linux" "arm64"
        ;;
    amd64)
        build_binary "linux" "amd64"
        ;;
    arm)
        build_binary "linux" "arm"
        ;;
esac

echo ""
echo -e "${GREEN}${BOLD}🎉 Build pipeline complete!${NC}"
