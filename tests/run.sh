#!/usr/bin/env bash
# Cross-check `gopapy dump` against CPython's ast.dump for every fixture
# under tests/grammar. Reports per-file PASS/SKIP/FAIL and exits non-zero if
# any fixture diverges.
#
# Two skip conditions:
#   1. Fixture requires newer Python than running (# Python 3.X+ comment).
#   2. Running Python is older than 3.13 — ast.dump() gained show_empty=False
#      in 3.13, so the format differs from gopapy's output on 3.12 and below.
#      Those versions skip the oracle comparison but still check that
#      `gopapy dump` runs without error.
set -u

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
oracle="$root/internal/oracle/oracle.py"

python_bin="${PYTHON:-python3}"
gopapy_bin="$root/bin/gopapy"

mkdir -p "$root/bin"
( cd "$root" && go build -o "$gopapy_bin" ./cmd/gopapy ) || exit 2

# Capture the running Python minor version once.
py_minor="$("$python_bin" -c 'import sys; print(sys.version_info.minor)')"

pass=0
skip=0
fail=0
for fixture in "$here"/grammar/*.py; do
    # Read minimum Python version from "# Python 3.X+" on the first line.
    first_line="$(head -1 "$fixture")"
    min_minor=""
    if [[ "$first_line" =~ ^#[[:space:]]*Python[[:space:]]+3\.([0-9]+)\+ ]]; then
        min_minor="${BASH_REMATCH[1]}"
    fi
    if [[ -n "$min_minor" && "$py_minor" -lt "$min_minor" ]]; then
        echo "SKIP $(basename "$fixture") (requires Python 3.$min_minor+, have 3.$py_minor)"
        skip=$((skip + 1))
        continue
    fi

    # On Python < 3.13, ast.dump() does not support show_empty=False and
    # always prints empty fields. Our ASTDump omits them (matching 3.13+).
    # Skip the oracle diff, but still verify gopapy dump does not crash.
    if [[ "$py_minor" -lt 13 ]]; then
        if "$gopapy_bin" dump "$fixture" > /dev/null 2>&1; then
            echo "PASS $(basename "$fixture") (dump-only, oracle skipped on 3.$py_minor)"
            pass=$((pass + 1))
        else
            echo "FAIL $(basename "$fixture") (gopapy dump error on 3.$py_minor)"
            "$gopapy_bin" dump "$fixture" 2>&1 | sed 's/^/    /'
            fail=$((fail + 1))
        fi
        continue
    fi

    want="$("$python_bin" "$oracle" "$fixture")" || {
        echo "ORACLE-ERR $fixture"
        fail=$((fail + 1))
        continue
    }
    got="$("$gopapy_bin" dump "$fixture")" || {
        echo "GOPAPY-ERR $fixture"
        fail=$((fail + 1))
        continue
    }
    if [ "$got" = "$want" ]; then
        echo "PASS $(basename "$fixture")"
        pass=$((pass + 1))
    else
        echo "FAIL $(basename "$fixture")"
        diff <(printf '%s\n' "$want") <(printf '%s\n' "$got") | sed 's/^/    /'
        fail=$((fail + 1))
    fi
done

echo
echo "$pass passed, $skip skipped, $fail failed"
[ "$fail" -eq 0 ]
