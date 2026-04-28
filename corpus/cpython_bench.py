#!/usr/bin/env python3
"""Measure CPython ast.parse throughput on the corpus.

Usage: python3 corpus/cpython_bench.py corpus/src

Pre-reads all .py files into memory, then runs ast.parse() in a timed loop.
Prints throughput in the same key: value format as `gopapy bench` so the
numbers are easy to compare side-by-side.

Runs two variants:
  cpython-serial:   single process, single thread (GIL-bound)
  cpython-parallel: multiprocessing.Pool sized to os.cpu_count()
"""

import ast
import os
import sys
import time
import multiprocessing


def _parse_one(data: bytes) -> bool:
    try:
        ast.parse(data)
        return True
    except Exception:
        return False


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

    chunks = [data for _path, data in contents]
    mb = total_bytes / 1_000_000
    print(f"cpython-files: {len(chunks)}")
    print(f"cpython-bytes: {mb:.1f} MB")

    # Warm-up pass: prime caches and avoid module-load overhead.
    for data in chunks[:200]:
        try:
            ast.parse(data)
        except Exception:
            pass

    # Serial timed pass.
    t0 = time.perf_counter()
    ok_serial = sum(1 for data in chunks if _parse_one(data))
    elapsed_serial = time.perf_counter() - t0
    print(
        f"cpython-serial-parse-rate: {ok_serial / elapsed_serial:.1f} files/s,"
        f" {mb / elapsed_serial:.1f} MB/s"
    )

    # Parallel timed pass using multiprocessing.
    workers = os.cpu_count() or 1
    t0 = time.perf_counter()
    with multiprocessing.Pool(workers) as pool:
        results = pool.map(_parse_one, chunks)
    elapsed_par = time.perf_counter() - t0
    ok_par = sum(results)
    print(
        f"cpython-parallel-parse-rate ({workers} workers):"
        f" {ok_par / elapsed_par:.1f} files/s,"
        f" {mb / elapsed_par:.1f} MB/s"
    )


if __name__ == "__main__":
    main()
