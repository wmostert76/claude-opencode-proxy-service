#!/usr/bin/env bash
set -euo pipefail

REPO="wmostert76/claude-go"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

BIN_DIR="$HOME/.local/bin"
mkdir -p "$BIN_DIR"

URL="https://github.com/${REPO}/releases/latest/download/claude-go-${OS}-${ARCH}"
echo "Downloading Claude Go for ${OS}/${ARCH}..."
curl -fsSL "$URL" -o "$BIN_DIR/claude-go"
chmod +x "$BIN_DIR/claude-go"

if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
  echo
  echo "Note: $BIN_DIR is not on your PATH."
  echo "Add this to your shell profile:"
  echo
  echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo
echo "Claude Go installed to $BIN_DIR/claude-go"
echo
echo "Next steps:"
echo "  claude-go install                # Install Claude Code locally"
echo "  claude-go --api <key>            # Store your OpenCode Go API key"
echo "  claude-go                        # Start!"
