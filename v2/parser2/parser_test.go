package parser2

import (
	"strings"
	"testing"
)

// Each row asserts the dumped tree shape for an input that v0.1.28
// supports. The dump format is parens-explicit so precedence
// regressions show up as visible structure changes.
var parseTable = []struct {
	name string
	src  string
	want string
}{
	// Literals.
	{"int_zero", "0", `Constant(value=0)`},
	{"int_positive", "42", `Constant(value=42)`},
	{"int_large", "1234567890", `Constant(value=1234567890)`},
	{"float_basic", "3.14", `Constant(value=3.14)`},
	{"float_no_int_part", ".5", `Constant(value=0.5)`},
	{"float_exp", "1e10", `Constant(value=1e+10)`},
	{"float_negexp", "1.5e-3", `Constant(value=0.0015)`},
	{"string_double", `"hello"`, `Constant(value="hello")`},
	{"string_single", `'world'`, `Constant(value="world")`},
	{"string_empty", `""`, `Constant(value="")`},
	{"string_escape", `"a\nb"`, `Constant(value="a\nb")`},
	{"none", "None", `Constant(value=None)`},
	{"true", "True", `Constant(value=True)`},
	{"false", "False", `Constant(value=False)`},

	// Names.
	{"name_short", "x", `Name(id="x")`},
	{"name_underscore", "_foo", `Name(id="_foo")`},
	{"name_mixed", "MyVar2", `Name(id="MyVar2")`},

	// Unary.
	{"neg_int", "-1", `UnaryOp(op=USub, operand=Constant(value=1))`},
	{"pos_int", "+2", `UnaryOp(op=UAdd, operand=Constant(value=2))`},
	{"invert_int", "~3", `UnaryOp(op=Invert, operand=Constant(value=3))`},
	{"not_name", "not x", `UnaryOp(op=Not, operand=Name(id="x"))`},
	{"double_neg", "--5", `UnaryOp(op=USub, operand=UnaryOp(op=USub, operand=Constant(value=5)))`},

	// Binary arithmetic, basic.
	{"add", "1 + 2", `BinOp(op=Add, left=Constant(value=1), right=Constant(value=2))`},
	{"sub", "5 - 3", `BinOp(op=Sub, left=Constant(value=5), right=Constant(value=3))`},
	{"mul", "4 * 6", `BinOp(op=Mult, left=Constant(value=4), right=Constant(value=6))`},
	{"div", "8 / 2", `BinOp(op=Div, left=Constant(value=8), right=Constant(value=2))`},

	// Precedence: * binds tighter than +.
	{
		"prec_mul_over_add",
		"1 + 2 * 3",
		`BinOp(op=Add, left=Constant(value=1), right=BinOp(op=Mult, left=Constant(value=2), right=Constant(value=3)))`,
	},
	{
		"prec_paren_overrides",
		"(1 + 2) * 3",
		`BinOp(op=Mult, left=BinOp(op=Add, left=Constant(value=1), right=Constant(value=2)), right=Constant(value=3))`,
	},
	{
		"prec_unary_over_mul",
		"-2 * 3",
		`BinOp(op=Mult, left=UnaryOp(op=USub, operand=Constant(value=2)), right=Constant(value=3))`,
	},

	// Left-associativity within a precedence level.
	{
		"left_assoc_sub",
		"10 - 3 - 2",
		`BinOp(op=Sub, left=BinOp(op=Sub, left=Constant(value=10), right=Constant(value=3)), right=Constant(value=2))`,
	},

	// Mixed.
	{
		"mixed_name_and_lit",
		"x + 1",
		`BinOp(op=Add, left=Name(id="x"), right=Constant(value=1))`,
	},
}

func TestParseExpression_Table(t *testing.T) {
	for _, row := range parseTable {
		t.Run(row.name, func(t *testing.T) {
			got, err := ParseExpression(row.src)
			if err != nil {
				t.Fatalf("ParseExpression(%q): %v", row.src, err)
			}
			dump := Dump(got)
			if dump != row.want {
				t.Errorf("ParseExpression(%q):\n  got:  %s\n  want: %s", row.src, dump, row.want)
			}
		})
	}
}

// Error cases. Each row asserts that the parser rejects the input
// rather than silently accepting garbage.
var errTable = []struct {
	name    string
	src     string
	wantSub string
}{
	{"empty", "", "unexpected token"},
	{"trailing_op", "1 +", "unexpected token"},
	{"bare_op", "*", "unexpected token"},
	{"unmatched_paren", "(1 + 2", "expected ')'"},
	{"trailing_garbage", "1 2", "unexpected token"},
	{"out_of_scope_at", "@", "unexpected character"},
	{"out_of_scope_double_star", "1 ** 2", "unexpected token"},
	{"unterminated_string", `"hello`, "unterminated string"},
}

func TestParseExpression_Errors(t *testing.T) {
	for _, row := range errTable {
		t.Run(row.name, func(t *testing.T) {
			_, err := ParseExpression(row.src)
			if err == nil {
				t.Fatalf("ParseExpression(%q): expected error, got nil", row.src)
			}
			if !strings.Contains(err.Error(), row.wantSub) {
				t.Errorf("ParseExpression(%q): error %q does not contain %q",
					row.src, err.Error(), row.wantSub)
			}
		})
	}
}

// TestParseExpression_PositionTracking sanity-checks that node Pos
// reflects 1-indexed line and 0-indexed column. This is the contract
// downstream consumers (linter, future converter to v1) depend on.
func TestParseExpression_PositionTracking(t *testing.T) {
	expr, err := ParseExpression("  x")
	if err != nil {
		t.Fatalf("ParseExpression: %v", err)
	}
	n, ok := expr.(*Name)
	if !ok {
		t.Fatalf("expected *Name, got %T", expr)
	}
	if n.P.Line != 1 || n.P.Col != 2 {
		t.Errorf("Pos = %+v, want {Line:1 Col:2}", n.P)
	}
}
