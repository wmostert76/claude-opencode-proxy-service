#!/usr/bin/env bash
set -Eeuo pipefail

readonly BIN_DIR="${NPM_GLOBAL_BIN:-$HOME/.local/share/npm-global/bin}"
readonly WRAPPER="$BIN_DIR/claude"
readonly REAL_CLAUDE="$HOME/.local/share/npm-global/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe"

if [[ ! -x "$REAL_CLAUDE" ]]; then
  echo "Error: Claude Code binary not found: $REAL_CLAUDE" >&2
  exit 1
fi

rm -f "$WRAPPER"
ln -s ../lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe "$WRAPPER"

echo "Restored direct Claude Code symlink:"
echo "  $WRAPPER -> $REAL_CLAUDE"
