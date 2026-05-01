#!/usr/bin/env bash
set -Eeuo pipefail

readonly WRAPPER="$HOME/.local/bin/claude-go"

if [[ ! -L "$WRAPPER" ]]; then
  echo "Claude Go not found: $WRAPPER" >&2
  exit 1
fi

rm -f "$WRAPPER"
echo "Claude Go uninstalled."
echo
echo "Your local Claude Code installation and config are untouched."
echo "To reinstall: curl -fsSL https://raw.githubusercontent.com/wmostert76/claude-go/main/scripts/bootstrap.sh | bash"
