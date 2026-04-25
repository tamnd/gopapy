#!/usr/bin/env python3
# Oracle: read a Python source file (or stdin) and print ast.dump(ast.parse(src))
# in CPython's default compact form. Used by tests/run.sh to diff against
# `gopapy dump`.
import ast
import sys


def main() -> int:
    if len(sys.argv) > 1:
        with open(sys.argv[1], "rb") as f:
            src = f.read()
    else:
        src = sys.stdin.buffer.read()
    tree = ast.parse(src)
    sys.stdout.write(ast.dump(tree))
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
