#!/usr/bin/env bash
set -Eeuo pipefail

readonly ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
readonly BIN_DIR="${NPM_GLOBAL_BIN:-$HOME/.local/share/npm-global/bin}"
readonly WRAPPER="$BIN_DIR/claude"
readonly TARGET="$ROOT_DIR/bin/claude-opencode"

mkdir -p "$BIN_DIR"

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
