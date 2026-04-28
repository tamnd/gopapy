#!/usr/bin/env python3
"""Measure CPython ast.parse throughput on the corpus.

Usage: python3 corpus/cpython_bench.py corpus/src

Pre-reads all .py files into memory, then runs ast.parse() in a timed loop.
Prints throughput in the same key: value format as `gopapy bench` so the
numbers are easy to compare side-by-side.
"""

import ast
import os
import sys
import time


def main():
    if len(sys.argv) < 2:
        print("usage: cpython_bench.py <src-dir>", file=sys.stderr)
        sys.exit(1)

    src_dir = sys.argv[1]

    # Collect all .py files.
    paths = []
    for root, _dirs, names in os.walk(src_dir):
        for name in names:
            if name.endswith(".py"):
                paths.append(os.path.join(root, name))
    paths.sort()

    # Pre-read into memory so disk I/O does not pollute the parse measurement.
    contents = []
    total_bytes = 0
    for path in paths:
        try:
            data = open(path, "rb").read()
        except OSError:
            continue
        contents.append((path, data))
        total_bytes += len(data)

    if not contents:
        print("no .py files found under", src_dir, file=sys.stderr)
        sys.exit(1)

    # Warm-up pass: prime caches and JIT (CPython has neither, but avoids
    # module-load overhead on the first few files).
    for _path, data in contents[:200]:
        try:
            ast.parse(data)
        except Exception:
            pass

    # Timed pass.
    t0 = time.perf_counter()
    ok = 0
    for _path, data in contents:
        try:
            ast.parse(data)
            ok += 1
        except Exception:
            pass
    elapsed = time.perf_counter() - t0

    mb = total_bytes / 1_000_000
    print(f"cpython-files: {ok}")
    print(f"cpython-bytes: {mb:.1f} MB")
    print(f"cpython-parse-rate: {ok / elapsed:.1f} files/s, {mb / elapsed:.1f} MB/s")


if __name__ == "__main__":
    main()
