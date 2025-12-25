#!/bin/bash

# 1. Build the Frontend
echo "🏗️  Building Frontend..."
cd web
npm install
npm run build
cd ..

# 2. Build the Backend (Cross-Compile for Linux ARM64)
# modernc.org/sqlite allows CGO_ENABLED=0, making cross-compilation easy.
echo "🐧 Building Backend for Linux ARM64..."
mkdir -p bin

# GOARCH=arm64 is for Raspberry Pi 3/4/5, Oracle Cloud Ampere, etc.
# Use GOARCH=arm for older 32-bit ARM boards.
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/papermc-manager-arm64 ./cmd/server/main.go

echo "✅ Build Complete!"
echo "📂 Binary location: bin/papermc-manager-arm64"

