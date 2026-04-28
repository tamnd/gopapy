#!/usr/bin/env bash
# corpus_diff.sh: sample N files from corpus/src/ and diff gopapy dump vs CPython ast.dump.
# Usage: corpus_diff.sh [sample_size] [seed]
# Defaults: sample_size=100, seed=$(date +%Y%m%d)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SRC_DIR="$REPO_ROOT/corpus/src"
GOPAPY="$REPO_ROOT/bin/gopapy"
SAMPLE_SIZE="${1:-100}"
SEED="${2:-$(date +%Y%m%d)}"
LOG_FILE="${REPO_ROOT}/corpus_diff.log"

if [ ! -d "$SRC_DIR" ]; then
  echo "corpus/src not found; run corpus/download.sh first" >&2
  exit 1
fi

if [ ! -x "$GOPAPY" ]; then
  echo "gopapy binary not found at $GOPAPY; run: go build -o bin/gopapy ./cmd/gopapy" >&2
  exit 1
fi

# Write sample file list to a temp file. Uses Python for seeded shuffle so the
# result is reproducible and portable across Linux and macOS.
SAMPLE_FILE="$(mktemp)"
trap 'rm -f "$SAMPLE_FILE"' EXIT

python3 - "$SRC_DIR" "$SEED" "$SAMPLE_SIZE" "$SAMPLE_FILE" <<'PYEOF'
import sys, random, os

src_dir, seed, n, out = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), sys.argv[4]
files = sorted(
    os.path.join(root, f)
    for root, _, names in os.walk(src_dir)
    for f in names
    if f.endswith(".py")
)
random.seed(seed)
random.shuffle(files)
with open(out, "w") as fh:
    fh.write("\n".join(files[:n]) + "\n")
PYEOF

total=$(wc -l < "$SAMPLE_FILE" | tr -d ' ')
echo "corpus-astdiff: seed=$SEED sample=$total files" | tee "$LOG_FILE"

pass=0
fail=0

while IFS= read -r f; do
  [ -z "$f" ] && continue
  gopapy_out=$("$GOPAPY" dump "$f" 2>/dev/null) || {
    echo "GOPAPY_ERROR $f" | tee -a "$LOG_FILE"
    fail=$((fail + 1))
    continue
  }
  cpython_out=$(python3 -c "import ast, sys; print(ast.dump(ast.parse(open(sys.argv[1]).read())))" "$f" 2>/dev/null) || {
    echo "CPYTHON_ERROR $f" | tee -a "$LOG_FILE"
    fail=$((fail + 1))
    continue
  }
  if [ "$gopapy_out" = "$cpython_out" ]; then
    pass=$((pass + 1))
  else
    echo "MISMATCH $f" | tee -a "$LOG_FILE"
    diff <(echo "$cpython_out") <(echo "$gopapy_out") >> "$LOG_FILE" || true
    fail=$((fail + 1))
  fi
done < "$SAMPLE_FILE"

echo "pass=$pass fail=$fail" | tee -a "$LOG_FILE"

if [ "$fail" -gt 0 ]; then
  exit 1
fi
