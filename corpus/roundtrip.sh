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

# Permanent exceptions — legacy-unparser fidelity gaps, not parse bugs.
# Both files are Black test fixtures (not user code). The new parser passes
# corpus-astdiff for both; only the legacy unparser's parenthesization and
# implicit-string-concat handling differ from CPython's unparser. Not worth
# fixing in the legacy path.
#
#   pep_572_remove_parens.py   walrus-operator paren removal (legacy unparser)
#   preview_long_strings.py    implicit string-concat prefix combos (legacy unparser)
ALLOW=(
  "$SRC_DIR/black-25.1.0/tests/data/cases/pep_572_remove_parens.py"
  "$SRC_DIR/black-25.1.0/tests/data/cases/preview_long_strings.py"
)

ALLOW_FLAGS=()
for f in "${ALLOW[@]}"; do
  ALLOW_FLAGS+=(--allow "$f")
done

echo "Running gopapy unparse --check on corpus/src/"
"$GOPAPY" unparse --check "${ALLOW_FLAGS[@]}" "$SRC_DIR"
