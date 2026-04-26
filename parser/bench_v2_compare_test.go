package parser

import "testing"

// benchExprCorpus mirrors v2/parser2/bench_test.go's benchCorpus so
// `go test -bench=BenchmarkParseExpressionV1Compare` here and
// `go test -bench=BenchmarkParseExpression` in v2/parser2 produce
// directly-comparable numbers. The PR for every parser2 release pastes
// both columns side-by-side.
//
// Keep this list in sync with v2/parser2/bench_test.go's benchCorpus.
var benchExprCorpus = []string{
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

func BenchmarkParseExpressionV1Compare(b *testing.B) {
	var total int
	for _, s := range benchExprCorpus {
		total += len(s)
	}
	b.SetBytes(int64(total))
	b.ResetTimer()
	for b.Loop() {
		for _, s := range benchExprCorpus {
			if _, err := ParseExpression(s); err != nil {
				b.Fatalf("ParseExpression(%q): %v", s, err)
			}
		}
	}
}
