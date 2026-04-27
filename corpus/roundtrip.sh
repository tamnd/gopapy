#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SRC_DIR="$SCRIPT_DIR/src"
GOPAPY="$REPO_ROOT/bin/gopapy"

if [ ! -d "$SRC_DIR" ]; then
  echo "corpus/src not found; run corpus/download.sh first"
  exit 1
fi

if [ ! -x "$GOPAPY" ]; then
  echo "gopapy binary not found at $GOPAPY; run: go build -o bin/gopapy ./cmd/gopapy"
  exit 1
fi

echo "Running gopapy unparse --check on corpus/src/"
"$GOPAPY" unparse --check "$SRC_DIR"
