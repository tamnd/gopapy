package parser

import "testing"

// benchExprCorpus mirrors v2/parser2/bench_test.go's benchCorpus so
// `go test -bench=BenchmarkParseExpressionV1Compare` here and
// `go test -bench=BenchmarkParseExpression` in v2/parser2 produce
// directly-comparable numbers. The PR for v0.1.28 (and every
// subsequent parser2 PR) pastes both columns side-by-side.
//
// Keep this list in sync with v2/parser2/bench_test.go's benchCorpus.
var benchExprCorpus = []string{
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
