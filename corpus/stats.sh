#!/usr/bin/env bash
# stats.sh: print corpus health metrics in a grep-friendly format.
# Run from the repo root after `go build -o bin/gopapy ./cmd/gopapy`
# and after corpus/download.sh has populated corpus/src/.
#
# Output example:
#   corpus-files: 12179
#   corpus-bytes: 82.3 MB
#   corpus-parse-rate: 1234.5 files/s, 8.3 MB/s
#   corpus-roundtrip-allowed: 2
#   corpus-astdiff-pass: 199
#   corpus-astdiff-skip: 1
#   corpus-astdiff-fail: 0
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GOPAPY="$REPO_ROOT/bin/gopapy"
SRC_DIR="$SCRIPT_DIR/src"
ROUNDTRIP_SH="$SCRIPT_DIR/roundtrip.sh"
DIFF_SH="$REPO_ROOT/internal/oracle/corpus_diff.sh"

if [ ! -x "$GOPAPY" ]; then
  echo "gopapy binary not found; run: go build -o bin/gopapy ./cmd/gopapy" >&2
  exit 1
fi
if [ ! -d "$SRC_DIR" ]; then
  echo "corpus/src not found; run corpus/download.sh first" >&2
  exit 1
fi

# Throughput via gopapy bench.
"$GOPAPY" bench "$SRC_DIR"

# Count entries in the ALLOW array in roundtrip.sh.
allowed=$(awk '/^ALLOW=\(/{found=1; next} found && /^\)/{exit} found && /SRC_DIR/{n++} END{print n+0}' "$ROUNDTRIP_SH")
echo "corpus-roundtrip-allowed: $allowed"

# Astdiff sample (100 files, seeded by date for reproducibility).
astdiff_out=$("$DIFF_SH" 100 2>/dev/null | tail -1)
pass=$(echo "$astdiff_out" | grep -o 'pass=[0-9]*' | cut -d= -f2)
skip=$(echo "$astdiff_out" | grep -o 'skip=[0-9]*' | cut -d= -f2)
fail=$(echo "$astdiff_out" | grep -o 'fail=[0-9]*' | cut -d= -f2)
echo "corpus-astdiff-pass: ${pass:-?}"
echo "corpus-astdiff-skip: ${skip:-?}"
echo "corpus-astdiff-fail: ${fail:-?}"
