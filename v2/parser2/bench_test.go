package parser2

import "testing"

// benchCorpus is the fixed set of expressions used for v1-vs-v2
// performance comparison. It exercises every form parser2 covers in
// v0.1.28 — literals, names, unary, binary arithmetic, parentheses,
// precedence corners. The same strings are benched against v1's
// parser.ParseExpression in parser/bench_v2_compare_test.go so the
// PR description can paste side-by-side numbers.
var benchCorpus = []string{
	"0",
	"42",
	"1234567890",
	"3.14",
	".5",
	"1e10",
	"1.5e-3",
	`"hello"`,
	`'world'`,
	`""`,
	"None",
	"True",
	"False",
	"x",
	"_foo",
	"MyVar2",
	"-1",
	"+2",
	"~3",
	"not x",
	"--5",
	"1 + 2",
	"5 - 3",
	"4 * 6",
	"8 / 2",
	"1 + 2 * 3",
	"(1 + 2) * 3",
	"-2 * 3",
	"10 - 3 - 2",
	"x + 1",
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
