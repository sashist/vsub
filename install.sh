#!/usr/bin/env sh
set -eu

REPO="${REPO:-sashist/vsub}"
BIN_NAME="vsub"

if [ -d "$HOME/.local/bin" ] || printf '%s' "$PATH" | grep -q "$HOME/.local/bin"; then
  BIN_DIR="$HOME/.local/bin"
  mkdir -p "$BIN_DIR"
elif [ -w "/usr/local/bin" ]; then
  BIN_DIR="/usr/local/bin"
else
  BIN_DIR="$HOME/.local/bin"
  mkdir -p "$BIN_DIR"
fi

DEST="$BIN_DIR/$BIN_NAME"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin) ;;
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

URL="https://github.com/$REPO/releases/latest/download/${BIN_NAME}-${OS}-${ARCH}"
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

echo "Downloading $URL..."
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"
mv "$TMP" "$DEST"

echo "Installed $DEST"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "Add $BIN_DIR to your PATH to run '$BIN_NAME' directly." ;;
esac
