#!/usr/bin/env bash
# install.sh — download and install ai-harness.
set -euo pipefail

OWNER="zacharyLYH"
REPO="ai-harness"
BINARY_NAME="ai-harness"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
esac

URL="https://github.com/$OWNER/$REPO/releases/latest/download/${BINARY_NAME}-${OS}-${ARCH}.tar.gz"

INSTALL_DIR="/usr/local/bin"
SUDO=""
if [ ! -w "$INSTALL_DIR" ]; then
  SUDO="sudo"
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading $BINARY_NAME ($OS/$ARCH)..."
curl -sSL -o "$TMP_DIR/$BINARY_NAME.tar.gz" "$URL"

echo "Installing to $INSTALL_DIR/$BINARY_NAME..."
tar -xzf "$TMP_DIR/$BINARY_NAME.tar.gz" -C "$TMP_DIR"
$SUDO install -m 0755 "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

echo "Done. Run '$BINARY_NAME' to start."
