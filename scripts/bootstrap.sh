#!/usr/bin/env bash
set -Eeuo pipefail

readonly REPO="${CLAUDE_OPENCODE_REPO:-wmostert76/claude-opencode-proxy-service}"
readonly INSTALL_DIR="${CLAUDE_OPENCODE_HOME:-$HOME/.local/share/claude-opencode-proxy-service}"
readonly ARCHIVE_URL="https://github.com/$REPO/archive/refs/heads/main.tar.gz"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Error: required command not found: $1" >&2
    exit 1
  }
}

need_cmd curl
need_cmd tar
need_cmd node
need_cmd npm
need_cmd python3

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

echo "Downloading $REPO..."
curl -fsSL "$ARCHIVE_URL" | tar -xz -C "$tmpdir" --strip-components=1

rm -rf "$INSTALL_DIR"
mkdir -p "$(dirname "$INSTALL_DIR")"
mv "$tmpdir" "$INSTALL_DIR"
trap - EXIT

chmod +x "$INSTALL_DIR/bin/claude-opencode" "$INSTALL_DIR/scripts/install.sh" "$INSTALL_DIR/scripts/uninstall.sh"

"$INSTALL_DIR/scripts/install.sh"

echo
echo "Installed Claude OpenCode Proxy Service in:"
echo "  $INSTALL_DIR"
