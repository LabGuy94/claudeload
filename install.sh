#!/usr/bin/env sh
set -euf

REPO="LabGuy94/claudeload"
BINARY_NAME="claudeload"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

TAG="${TAG:-latest}"

if [ "$TAG" = "latest" ]; then
  API_URL="https://api.github.com/repos/$REPO/releases/latest"
else
  API_URL="https://api.github.com/repos/$REPO/releases/tags/$TAG"
fi

if command -v curl >/dev/null 2>&1; then
  GET="curl -fsSL"
elif command -v wget >/dev/null 2>&1; then
  GET="wget -qO-"
else
  echo "Missing curl or wget"
  exit 1
fi

RELEASE_JSON="$($GET "$API_URL")"
RELEASE_TAG="$(printf "%s" "$RELEASE_JSON" | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p' | head -n 1)"
if [ -z "$RELEASE_TAG" ]; then
  RELEASE_TAG="$TAG"
fi

ASSET_NAME="claudeload_${RELEASE_TAG#v}_${OS}_${ARCH}"
ARCHIVE_TGZ="${ASSET_NAME}.tar.gz"
ARCHIVE_URL="$(printf "%s" "$RELEASE_JSON" | \
  sed -n 's/.*"browser_download_url": "\(.*'${ARCHIVE_TGZ}'\)".*/\1/p' | head -n 1)"

if [ -z "$ARCHIVE_URL" ]; then
  echo "Could not find release asset for $OS/$ARCH (tag: $TAG)"
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading $ARCHIVE_TGZ..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$ARCHIVE_URL" -o "$TMP_DIR/$ARCHIVE_TGZ"
else
  wget -qO "$TMP_DIR/$ARCHIVE_TGZ" "$ARCHIVE_URL"
fi

mkdir -p "$INSTALL_DIR"
tar -xzf "$TMP_DIR/$ARCHIVE_TGZ" -C "$TMP_DIR"

if [ ! -f "$TMP_DIR/$BINARY_NAME" ]; then
  echo "Binary not found in archive"
  exit 1
fi

mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo "Installed to $INSTALL_DIR/$BINARY_NAME"
echo "Make sure $INSTALL_DIR is in your PATH"
