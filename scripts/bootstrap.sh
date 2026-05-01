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

# Install shell completions
if [ -d "$HOME/.config/fish/completions" ]; then
	"$BIN_DIR/claude-go" --completion fish > "$HOME/.config/fish/completions/claude-go.fish" 2>/dev/null || true
	echo "Fish completions installed"
fi
if [ -d "$HOME/.local/share/bash-completion/completions" ]; then
	"$BIN_DIR/claude-go" --completion bash > "$HOME/.local/share/bash-completion/completions/claude-go" 2>/dev/null || true
	echo "Bash completions installed"
fi

if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
  SHELL_NAME="$(basename "$SHELL")"
  RC_FILE=""
  case "$SHELL_NAME" in
    zsh)  RC_FILE="$HOME/.zshrc" ;;
    bash) RC_FILE="$HOME/.bashrc" ;;
    fish) RC_FILE="$HOME/.config/fish/config.fish" ;;
  esac

  if [[ -n "$RC_FILE" ]]; then
    mkdir -p "$(dirname "$RC_FILE")"
    if ! grep -q '$HOME/.local/bin' "$RC_FILE" 2>/dev/null; then
      echo "export PATH=\"\$HOME/.local/bin:\$PATH\"" >> "$RC_FILE"
      echo
      echo "Added ~/.local/bin to PATH in $RC_FILE"
      echo "Restart your terminal or run:  export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
  else
    echo
    echo "Note: $BIN_DIR is not on your PATH."
    echo "Add this to your shell profile:"
    echo
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
fi

echo
echo "Claude Go installed to $BIN_DIR/claude-go"
echo
echo "Next steps:"
echo "  claude-go install                # Install Claude Code locally"
echo "  claude-go --api <key>            # Store your OpenCode Go API key"
echo "  claude-go                        # Start!"
