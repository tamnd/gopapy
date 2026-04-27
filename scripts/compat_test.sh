#!/usr/bin/env bash
# Run gopapy check, unparse --check, and grammar fixture oracle for each
# Python version in the compat matrix (3.9-3.13). Uses uv to locate
# the installed Python for each version.
#
# Exit code: 0 if every version passes, 1 if any version has failures.
set -u

root="$(cd "$(dirname "$0")/.." && pwd)"
gopapy="$root/bin/gopapy"

VERSIONS=(3.9 3.10 3.11 3.12 3.13)

overall=0

for ver in "${VERSIONS[@]}"; do
    python_bin="$(uv python find "$ver" 2>/dev/null)" || {
        echo "==> SKIP Python $ver (not installed)"
        continue
    }
    echo ""
    echo "==> Python $ver  ($python_bin)"
    "$python_bin" --version

    PYDIR="$("$python_bin" -c "import sysconfig; print(sysconfig.get_paths()['stdlib'])")"
    echo "    stdlib: $PYDIR"

    fail_v=0

    echo "    -- gopapy check --"
    if ! "$gopapy" check "$PYDIR"; then
        echo "    FAIL: gopapy check found errors"
        fail_v=1
    fi

    echo "    -- gopapy unparse --check --"
    if ! "$gopapy" unparse --check "$PYDIR"; then
        echo "    FAIL: gopapy unparse --check found errors"
        fail_v=1
    fi

    echo "    -- grammar fixtures oracle --"
    if ! PYTHON="$python_bin" "$root/tests/run.sh"; then
        echo "    FAIL: grammar fixture oracle mismatch"
        fail_v=1
    fi

    if [ "$fail_v" -eq 0 ]; then
        echo "==> PASS Python $ver"
    else
        echo "==> FAIL Python $ver"
        overall=1
    fi
done

echo ""
if [ "$overall" -eq 0 ]; then
    echo "All versions passed."
else
    echo "One or more versions failed."
fi
exit "$overall"
