#!/usr/bin/env bash
# Cross-check `gopapy dump` against CPython's ast.dump for every fixture
# under tests/grammar. Reports per-file PASS/FAIL and exits non-zero if any
# fixture diverges.
set -u

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
oracle="$root/internal/oracle/oracle.py"

python_bin="${PYTHON:-python3}"
gopapy_bin="$root/bin/gopapy"

mkdir -p "$root/bin"
( cd "$root" && go build -o "$gopapy_bin" ./cmd/gopapy ) || exit 2

pass=0
fail=0
for fixture in "$here"/grammar/*.py; do
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
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
