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

ARCHIVE="$TMP_DIR/$BINARY_NAME.tar.gz"

echo "Downloading $BINARY_NAME ($OS/$ARCH)..."
# -f makes curl fail (non-zero) on HTTP errors instead of saving an error page.
if ! curl -fsSL -o "$ARCHIVE" "$URL"; then
  echo "Error: failed to download $URL" >&2
  echo "A release for $OS/$ARCH may not exist yet. See https://github.com/$OWNER/$REPO/releases" >&2
  exit 1
fi

# Guard against getting an HTML error page instead of a real archive.
if ! gzip -t "$ARCHIVE" 2>/dev/null; then
  echo "Error: downloaded file is not a valid archive (release may not be ready)." >&2
  echo "Check https://github.com/$OWNER/$REPO/releases for $OS/$ARCH." >&2
  exit 1
fi

echo "Installing to $INSTALL_DIR/$BINARY_NAME..."
tar -xzf "$ARCHIVE" -C "$TMP_DIR"
$SUDO install -m 0755 "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

echo "Done. Run '$BINARY_NAME' to start."
