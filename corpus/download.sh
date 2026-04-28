#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGES_FILE="$SCRIPT_DIR/packages.txt"
SRC_DIR="$SCRIPT_DIR/src"
SDIST_DIR="$SCRIPT_DIR/sdists"

if [ -d "$SRC_DIR" ] && [ -n "$(ls -A "$SRC_DIR" 2>/dev/null)" ]; then
  echo "corpus/src already populated, skipping download"
  exit 0
fi

mkdir -p "$SDIST_DIR" "$SRC_DIR"

while IFS= read -r pkg || [ -n "$pkg" ]; do
  [ -z "$pkg" ] && continue
  echo "Downloading $pkg"
  # --prefer-binary: use pre-built wheels when available; fall back to sdist.
  # This avoids needing C headers for packages like lxml that require them to
  # build from source. We only want the .py files regardless of wheel or sdist.
  python3 -m pip download --no-deps --prefer-binary -d "$SDIST_DIR" "$pkg"
done < "$PACKAGES_FILE"

# Extract .py files from sdist tarballs.
for archive in "$SDIST_DIR"/*.tar.gz; do
  [ -e "$archive" ] || continue
  echo "Extracting $archive"
  tmpdir=$(mktemp -d)
  tar -xzf "$archive" -C "$tmpdir"
  find "$tmpdir" -name "*.py" | while IFS= read -r f; do
    rel="${f#"$tmpdir/"}"
    dest="$SRC_DIR/$rel"
    mkdir -p "$(dirname "$dest")"
    cp "$f" "$dest"
  done
  rm -rf "$tmpdir"
done

# Extract .py files from wheels (zip format).
for wheel in "$SDIST_DIR"/*.whl; do
  [ -e "$wheel" ] || continue
  echo "Extracting $wheel"
  tmpdir=$(mktemp -d)
  python3 -c "import zipfile, sys; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])" "$wheel" "$tmpdir"
  find "$tmpdir" -name "*.py" | while IFS= read -r f; do
    rel="${f#"$tmpdir/"}"
    dest="$SRC_DIR/$rel"
    mkdir -p "$(dirname "$dest")"
    cp "$f" "$dest"
  done
  rm -rf "$tmpdir"
done

py_count=$(find "$SRC_DIR" -name "*.py" | wc -l)
echo "corpus/src populated: $py_count .py files"
