package ast

import (
	"testing"

	"github.com/tamnd/gopapy/parser"
)

// TestEndPos_NonZeroSpans asserts that the End* fields populate to a real
// position rather than mirroring the start. The exact end values come from
// participle (the start position of the token immediately following the
// construct), so the assertion is intentionally weak: we only require that
// (EndLineno, EndColOffset) > (Lineno, ColOffset). True CPython-faithful
// end positions need a hand-written parser; that's a v0.2.x follow-up.
func TestEndPos_NonZeroSpans(t *testing.T) {
	cases := []struct {
		name string
		src  string
		// pick is "first compound statement" (the test only inspects the
		// outermost body[0] node) — every fixture is shaped so body[0] is
		// the construct under test.
	}{
		// Multi-line constructs: end_lineno must exceed start lineno.
		{"func_def", "def f(x):\n    return x\n"},
		{"class_def", "class C:\n    x = 1\n"},
		{"if_else", "if x:\n    a = 1\nelse:\n    b = 2\n"},
		{"try_except", "try:\n    a = 1\nexcept E:\n    b = 2\n"},
		{"for_loop", "for x in xs:\n    a = x\n"},
		{"while_loop", "while x:\n    a = 1\n"},
		{"with_stmt", "with f() as x:\n    a = 1\n"},
		{"async_def", "async def f():\n    pass\n"},
		{"list_multiline", "x = [\n    1,\n    2,\n]\n"},
		{"dict_multiline", "x = {\n    1: 2,\n    3: 4,\n}\n"},

		// Multi-column constructs: same line, end_col_offset must exceed
		// start col_offset.
		{"return_value", "return 1 + 2\n"},
		{"call_args", "f(a, b, c)\n"},
		{"binop_chain", "a + b + c + d\n"},
		{"compare_chain", "a < b < c\n"},
		{"subscript", "xs[0]\n"},
		{"attribute_chain", "a.b.c.d\n"},
		{"tuple_literal", "(1, 2, 3)\n"},
		{"list_literal", "[1, 2, 3]\n"},
		{"dict_literal", "{1: 2, 3: 4}\n"},
		{"lambda", "lambda x: x + 1\n"},
		{"ifexp", "a if c else b\n"},
		{"unary_minus", "-(x + 1)\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseString("<test>", tc.src)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			mod := FromFile(f)
			if len(mod.Body) == 0 {
				t.Fatalf("empty body")
			}
			node := mod.Body[0]
			p := node.GetPos()
			start := position{p.Lineno, p.ColOffset}
			end := position{p.EndLineno, p.EndColOffset}
			if !end.after(start) {
				t.Errorf("end position not after start: start=%v end=%v src=%q",
					start, end, tc.src)
			}
		})
	}
}

type position struct{ line, col int }

func (a position) after(b position) bool {
	if a.line != b.line {
		return a.line > b.line
	}
	return a.col > b.col
}
