package parser2

import "testing"

// benchCorpus is the fixed set of expressions used for v1-vs-v2
// performance comparison. As of v0.1.29 it spans every expression
// form parser2 supports: literals, names, unary, all binops with
// precedence corners, comparisons, boolean ops, attribute,
// subscript, slices, calls, collection literals, comprehensions,
// lambdas, walrus. The same strings are benched against v1's
// parser.ParseExpression in parser/bench_v2_compare_test.go so the
// PR description can paste side-by-side numbers.
var benchCorpus = []string{
	// literals
	"0", "42", "1234567890", "1_000_000", "0xFF", "0o17", "0b1010",
	"3.14", ".5", "1e10", "1.5e-3", "3j",
	`"hello"`, `'world'`, `""`, `b"bytes"`, "...",
	"None", "True", "False",
	// names
	"x", "_foo", "MyVar2",
	// unary
	"-1", "+2", "~3", "not x", "--5",
	// arithmetic
	"1 + 2", "5 - 3", "4 * 6", "8 / 2", "9 // 4", "9 % 4",
	"2 ** 8", "2 ** 3 ** 4", "-2 ** 2",
	"1 + 2 * 3", "(1 + 2) * 3", "10 - 3 - 2",
	// bitwise
	"a | b", "a & b", "a ^ b", "1 << 4", "16 >> 2",
	"a | b & c",
	// comparisons
	"1 < 2", "1 < 2 < 3", "a == b != c",
	"x in xs", "x not in xs", "x is not None",
	// boolean
	"a or b", "a and b", "a or b or c", "a or b and c",
	"not a and b",
	// conditional
	"a if cond else b", "a if c1 else b if c2 else d",
	// attribute / subscript / call
	"a.b", "a.b.c", "a[1]", "a[1:2:3]", "a[:]", "a[::2]",
	"f()", "f(1)", "f(x=1, y=2)", "f(*xs, **kw)",
	"obj.method(arg)",
	// collections
	"[]", "()", "{}", "[1, 2, 3]", "(1, 2, 3)", "{1, 2, 3}",
	`{"a": 1, "b": 2}`, `{**other, "k": v}`, "[*xs, y]",
	// comprehensions
	"[x for x in xs]", "[x for x in xs if x > 0]",
	"{x for x in xs}", "{k: v for k, v in items}",
	"(x for x in xs)",
	// lambda
	"lambda: 1", "lambda x: x + 1", "lambda x, y=2: x + y",
	"lambda *args, **kw: args",
	// walrus
	"(x := 5)",
}

func BenchmarkParseExpression(b *testing.B) {
	var total int
	for _, s := range benchCorpus {
		total += len(s)
	}
	b.SetBytes(int64(total))
	b.ResetTimer()
	for b.Loop() {
		for _, s := range benchCorpus {
			if _, err := ParseExpression(s); err != nil {
				b.Fatalf("ParseExpression(%q): %v", s, err)
			}
		}
	}
}

// fileBenchSrc is a representative module-level Python source used to
// bench ParseFile end-to-end. Mirrored verbatim in
// parser/bench_v2_compare_test.go so v1 and v2 are timed against the
// same input.
const fileBenchSrc = `# module-level bench fixture
import os
import sys as _sys
from typing import List, Optional

CONST = 42
PI: float = 3.14

def add(a, b):
    return a + b

def fib(n: int) -> int:
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)

@cache
def cached(x):
    return x * 2

class Point:
    x: int = 0
    y: int = 0

    def __init__(self, x, y):
        self.x = x
        self.y = y

    def distance(self, other):
        dx = self.x - other.x
        dy = self.y - other.y
        return (dx * dx + dy * dy) ** 0.5

class Stream(Base, metaclass=Meta):
    def __init__(self):
        self.items = []

    def push(self, item):
        self.items.append(item)
        return self

async def fetch(url):
    async with session.get(url) as r:
        return await r.text()

async def gather(stream):
    async for chunk in stream:
        yield chunk

def process(items):
    total = 0
    for i, x in enumerate(items):
        if x is None:
            continue
        try:
            total += int(x)
        except ValueError as e:
            raise RuntimeError("bad value") from e
        finally:
            pass
    return total

def comp_demo(xs):
    squares = [x * x for x in xs if x > 0]
    pairs = {k: v for k, v in zip(xs, xs)}
    g = (x for x in xs)
    return squares, pairs, g

def main():
    while True:
        x, y = 1, 2
        a = b = c = 0
        x += 1
        del a
        assert x > 0, "must be positive"
        global CONST
        if x:
            pass
        elif y:
            pass
        else:
            pass
        with open("f") as f, open("g") as g:
            pass
        break

def fstring_demo(name, count):
    log(f"hi {name!r}, count={count:>5}")
    log(f"raw: {{ {name} }} done")
    return t"name={name} count={count}"

def match_demo(node):
    match node:
        case 0 | 1 | 2:
            return "small"
        case [a, *rest]:
            return rest
        case {"k": v, **rest}:
            return v
        case Point(x=0, y=0):
            return "origin"
        case Point(x, y) if x == y:
            return "diag"
        case _:
            return "other"

type Vector = list[float]

def first[T](xs: list[T]) -> T:
    return xs[0]

class Box[T]:
    def __init__(self, x: T) -> None:
        self.x = x
`

func BenchmarkParseFile(b *testing.B) {
	b.SetBytes(int64(len(fileBenchSrc)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := ParseFile("bench.py", fileBenchSrc); err != nil {
			b.Fatalf("ParseFile: %v", err)
		}
	}
}
