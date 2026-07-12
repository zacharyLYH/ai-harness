#!/usr/bin/env bash
# build.sh — cross-compile ai-harness for all supported platforms.
set -euo pipefail

BINARY_NAME="ai-harness"
BUILD_DIR="dist"

PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

for PLATFORM in "${PLATFORMS[@]}"; do
  OS="${PLATFORM%/*}"
  ARCH="${PLATFORM#*/}"
  OUTPUT="$BINARY_NAME"
  [ "$OS" = "windows" ] && OUTPUT="$OUTPUT.exe"

  echo "Building $OS/$ARCH..."
  CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" \
    go build -ldflags="-s -w" -o "$BUILD_DIR/$OUTPUT" .

  echo "Packaging ${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
  tar -czf "$BUILD_DIR/${BINARY_NAME}-${OS}-${ARCH}.tar.gz" -C "$BUILD_DIR" "$OUTPUT"
  rm -f "$BUILD_DIR/$OUTPUT"
done

echo "Build complete. Artifacts in ./$BUILD_DIR"
