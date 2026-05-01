#!/usr/bin/env bash
set -Eeuo pipefail

readonly ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
readonly BIN_DIR="$HOME/.local/bin"
readonly WRAPPER="$BIN_DIR/claude-go"
readonly TARGET="$ROOT_DIR/bin/claude-opencode"
readonly CLAUDE_CODE_DIR="$ROOT_DIR/node_modules/@anthropic-ai/claude-code"
readonly REAL_CLAUDE="$CLAUDE_CODE_DIR/bin/claude.exe"
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
  echo "Claude Go"
  echo "─────────────────────────────"
  printf '  %-12s v%s\n' "Release" "$(version)"
  printf '  %-12s %s\n' "Install" "$ROOT_DIR"
  printf '  %-12s %s\n' "Wrapper" "$WRAPPER"
  echo "─────────────────────────────"
  echo
}

ensure_local_bin() {
  mkdir -p "$BIN_DIR"

  if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    echo "Note: $BIN_DIR is not on your PATH."
    echo "Add this to your shell profile:"
    echo
    echo '  export PATH="$HOME/.local/bin:$PATH"'
    echo
  fi
}

ensure_claude_code() {
  if [[ -x "$REAL_CLAUDE" ]]; then
    return 0
  fi

  echo "Installing Claude Code locally in $ROOT_DIR..."

  cat > "$ROOT_DIR/package.json" <<PKG
{
  "name": "claude-go-runtime",
  "private": true,
  "dependencies": {
    "@anthropic-ai/claude-code": "*"
  }
}
PKG

  (cd "$ROOT_DIR" && npm install --no-audit --no-fund 2>&1)

  [[ -x "$REAL_CLAUDE" ]] || {
    echo "Error: Claude Code install finished, but binary was not found: $REAL_CLAUDE" >&2
    exit 1
  }

  echo "Claude Code installed locally."
}

print_header

need_cmd node
need_cmd npm
need_cmd python3
need_cmd curl

ensure_local_bin
ensure_claude_code

if [[ ! -x "$TARGET" ]]; then
  echo "Error: wrapper not executable: $TARGET" >&2
  exit 1
fi

rm -f "$WRAPPER"

cat > "$WRAPPER" <<EOF
#!/usr/bin/env bash
exec "$TARGET" "\$@"
EOF

chmod +x "$WRAPPER"

echo "Installed Claude Go wrapper:"
echo "  $WRAPPER -> $TARGET"
echo
echo "Next step:"
echo "  claude-go --api <opencode-go-api-key>"
