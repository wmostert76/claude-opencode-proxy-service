#!/usr/bin/env bash
set -Eeuo pipefail

readonly ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
readonly BIN_DIR="${NPM_GLOBAL_BIN:-$HOME/.local/share/npm-global/bin}"
readonly WRAPPER="$BIN_DIR/claude"
readonly TARGET="$ROOT_DIR/bin/claude-opencode"
readonly REAL_CLAUDE="$HOME/.local/share/npm-global/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe"
readonly VERSION_FILE="$ROOT_DIR/VERSION"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Error: required command not found: $1" >&2
    exit 1
  }
}

version() {
  if [[ -r "$VERSION_FILE" ]]; then
    tr -d '[:space:]' < "$VERSION_FILE"
  else
    printf 'dev'
  fi
}

print_header() {
  echo "Claude OpenCode Proxy Service"
  echo "─────────────────────────────"
  printf '  %-10s v%s\n' "Release" "$(version)"
  printf '  %-10s %s\n' "Install" "$ROOT_DIR"
  printf '  %-10s %s\n' "Wrapper" "$WRAPPER"
  echo "─────────────────────────────"
  echo
}

ensure_npm_prefix() {
  mkdir -p "$BIN_DIR"

  if [[ "$(npm config get prefix)" != "$HOME/.local/share/npm-global" ]]; then
    npm config set prefix "$HOME/.local/share/npm-global"
  fi
}

ensure_claude_code() {
  if [[ -x "$REAL_CLAUDE" ]]; then
    return 0
  fi

  echo "Claude Code not found; installing @anthropic-ai/claude-code..."
  npm install -g @anthropic-ai/claude-code

  [[ -x "$REAL_CLAUDE" ]] || {
    echo "Error: Claude Code install finished, but binary was not found: $REAL_CLAUDE" >&2
    exit 1
  }
}

print_header

need_cmd node
need_cmd npm
need_cmd python3
need_cmd curl

ensure_npm_prefix
ensure_claude_code

if [[ ! -x "$TARGET" ]]; then
  echo "Error: wrapper not executable: $TARGET" >&2
  exit 1
fi

if [[ -e "$WRAPPER" && ! -L "$WRAPPER" && ! -f "$WRAPPER" ]]; then
  echo "Error: refusing to replace non-regular file: $WRAPPER" >&2
  exit 1
fi

cat > "$WRAPPER" <<EOF
#!/usr/bin/env bash
exec "$TARGET" "\$@"
EOF

chmod +x "$WRAPPER"

echo "Installed Claude OpenCode wrapper:"
echo "  $WRAPPER -> $TARGET"
echo
echo "Next setup step:"
echo "  claude --api <opencode-go-api-key>"
