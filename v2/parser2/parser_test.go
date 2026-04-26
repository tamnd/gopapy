package parser2

import (
	"strings"
	"testing"
)

// Each row asserts the dumped tree shape for an input that v0.1.29
// supports. The dump format is parens-explicit so precedence
// regressions show up as visible structure changes.
var parseTable = []struct {
	name string
	src  string
	want string
}{
	// ----- Literals -----
	{"int_zero", "0", `Constant(value=0)`},
	{"int_positive", "42", `Constant(value=42)`},
	{"int_underscore", "1_000_000", `Constant(value=1000000)`},
	{"int_hex", "0xFF", `Constant(value=255)`},
	{"int_oct", "0o17", `Constant(value=15)`},
	{"int_bin", "0b1010", `Constant(value=10)`},
	{"float_basic", "3.14", `Constant(value=3.14)`},
	{"float_no_int_part", ".5", `Constant(value=0.5)`},
	{"float_exp", "1e10", `Constant(value=1e+10)`},
	{"float_negexp", "1.5e-3", `Constant(value=0.0015)`},
	{"complex", "3j", `Constant(value=3j)`},
	{"string_double", `"hello"`, `Constant(value="hello")`},
	{"string_single", `'world'`, `Constant(value="world")`},
	{"string_empty", `""`, `Constant(value="")`},
	{"string_escape", `"a\nb"`, `Constant(value="a\nb")`},
	{"string_concat", `"a" "b"`, `Constant(value="ab")`},
	{"string_triple", `"""hi"""`, `Constant(value="hi")`},
	{"bytes", `b"bytes"`, `Constant(value=b"bytes")`},
	{"raw", `r"a\nb"`, `Constant(value="a\\nb")`},
	{"none", "None", `Constant(value=None)`},
	{"true", "True", `Constant(value=True)`},
	{"false", "False", `Constant(value=False)`},
	{"ellipsis", "...", `Constant(value=...)`},

	// ----- Names -----
	{"name_short", "x", `Name(id="x")`},
	{"name_underscore", "_foo", `Name(id="_foo")`},
	{"name_mixed", "MyVar2", `Name(id="MyVar2")`},

	// ----- Unary -----
	{"neg_int", "-1", `UnaryOp(op=USub, operand=Constant(value=1))`},
	{"pos_int", "+2", `UnaryOp(op=UAdd, operand=Constant(value=2))`},
	{"invert_int", "~3", `UnaryOp(op=Invert, operand=Constant(value=3))`},
	{"not_name", "not x", `UnaryOp(op=Not, operand=Name(id="x"))`},
	{"double_neg", "--5", `UnaryOp(op=USub, operand=UnaryOp(op=USub, operand=Constant(value=5)))`},

	// ----- Arithmetic with full precedence ladder -----
	{"add", "1 + 2", `BinOp(op=Add, left=Constant(value=1), right=Constant(value=2))`},
	{"sub", "5 - 3", `BinOp(op=Sub, left=Constant(value=5), right=Constant(value=3))`},
	{"mul", "4 * 6", `BinOp(op=Mult, left=Constant(value=4), right=Constant(value=6))`},
	{"div", "8 / 2", `BinOp(op=Div, left=Constant(value=8), right=Constant(value=2))`},
	{"floordiv", "9 // 4", `BinOp(op=FloorDiv, left=Constant(value=9), right=Constant(value=4))`},
	{"mod", "9 % 4", `BinOp(op=Mod, left=Constant(value=9), right=Constant(value=4))`},
	{"matmult", "a @ b", `BinOp(op=MatMult, left=Name(id="a"), right=Name(id="b"))`},
	{"power", "2 ** 8", `BinOp(op=Pow, left=Constant(value=2), right=Constant(value=8))`},
	{
		"power_right_assoc",
		"2 ** 3 ** 4",
		`BinOp(op=Pow, left=Constant(value=2), right=BinOp(op=Pow, left=Constant(value=3), right=Constant(value=4)))`,
	},
	{
		"unary_outside_power",
		"-2 ** 2",
		`UnaryOp(op=USub, operand=BinOp(op=Pow, left=Constant(value=2), right=Constant(value=2)))`,
	},
	{
		"power_with_unary_rhs",
		"2 ** -1",
		`BinOp(op=Pow, left=Constant(value=2), right=UnaryOp(op=USub, operand=Constant(value=1)))`,
	},
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
		"left_assoc_sub",
		"10 - 3 - 2",
		`BinOp(op=Sub, left=BinOp(op=Sub, left=Constant(value=10), right=Constant(value=3)), right=Constant(value=2))`,
	},

	// ----- Bitwise + shift -----
	{"bitor", "a | b", `BinOp(op=BitOr, left=Name(id="a"), right=Name(id="b"))`},
	{"bitand", "a & b", `BinOp(op=BitAnd, left=Name(id="a"), right=Name(id="b"))`},
	{"bitxor", "a ^ b", `BinOp(op=BitXor, left=Name(id="a"), right=Name(id="b"))`},
	{"lshift", "1 << 4", `BinOp(op=LShift, left=Constant(value=1), right=Constant(value=4))`},
	{"rshift", "16 >> 2", `BinOp(op=RShift, left=Constant(value=16), right=Constant(value=2))`},
	{
		"bitor_lower_than_bitand",
		"a | b & c",
		`BinOp(op=BitOr, left=Name(id="a"), right=BinOp(op=BitAnd, left=Name(id="b"), right=Name(id="c")))`,
	},

	// ----- Comparisons (chained) -----
	{
		"compare_lt",
		"1 < 2",
		`Compare(left=Constant(value=1), ops=[Lt], comparators=[Constant(value=2)])`,
	},
	{
		"compare_chain",
		"1 < 2 < 3",
		`Compare(left=Constant(value=1), ops=[Lt, Lt], comparators=[Constant(value=2), Constant(value=3)])`,
	},
	{
		"compare_eq_neq",
		"a == b != c",
		`Compare(left=Name(id="a"), ops=[Eq, NotEq], comparators=[Name(id="b"), Name(id="c")])`,
	},
	{
		"compare_in",
		"x in xs",
		`Compare(left=Name(id="x"), ops=[In], comparators=[Name(id="xs")])`,
	},
	{
		"compare_not_in",
		"x not in xs",
		`Compare(left=Name(id="x"), ops=[NotIn], comparators=[Name(id="xs")])`,
	},
	{
		"compare_is_not",
		"x is not None",
		`Compare(left=Name(id="x"), ops=[IsNot], comparators=[Constant(value=None)])`,
	},

	// ----- Boolean ops -----
	{
		"or_simple",
		"a or b",
		`BoolOp(op=Or, values=[Name(id="a"), Name(id="b")])`,
	},
	{
		"and_simple",
		"a and b",
		`BoolOp(op=And, values=[Name(id="a"), Name(id="b")])`,
	},
	{
		"or_flattens",
		"a or b or c",
		`BoolOp(op=Or, values=[Name(id="a"), Name(id="b"), Name(id="c")])`,
	},
	{
		"and_or_precedence",
		"a or b and c",
		`BoolOp(op=Or, values=[Name(id="a"), BoolOp(op=And, values=[Name(id="b"), Name(id="c")])])`,
	},
	{
		"not_and_or",
		"not a and b",
		`BoolOp(op=And, values=[UnaryOp(op=Not, operand=Name(id="a")), Name(id="b")])`,
	},

	// ----- Conditional expression -----
	{
		"ifexp_basic",
		"a if cond else b",
		`IfExp(test=Name(id="cond"), body=Name(id="a"), orelse=Name(id="b"))`,
	},
	{
		"ifexp_nested",
		"a if c1 else b if c2 else d",
		`IfExp(test=Name(id="c1"), body=Name(id="a"), orelse=IfExp(test=Name(id="c2"), body=Name(id="b"), orelse=Name(id="d")))`,
	},

	// ----- Attribute, subscript, call -----
	{"attr", "a.b", `Attribute(value=Name(id="a"), attr="b")`},
	{
		"attr_chain",
		"a.b.c",
		`Attribute(value=Attribute(value=Name(id="a"), attr="b"), attr="c")`,
	},
	{
		"subscript_simple",
		"a[1]",
		`Subscript(value=Name(id="a"), slice=Constant(value=1))`,
	},
	{
		"slice_full",
		"a[1:2:3]",
		`Subscript(value=Name(id="a"), slice=Slice(lower=Constant(value=1), upper=Constant(value=2), step=Constant(value=3)))`,
	},
	{
		"slice_empty",
		"a[:]",
		`Subscript(value=Name(id="a"), slice=Slice(lower=nil, upper=nil, step=nil))`,
	},
	{
		"slice_step_only",
		"a[::2]",
		`Subscript(value=Name(id="a"), slice=Slice(lower=nil, upper=nil, step=Constant(value=2)))`,
	},
	{
		"call_no_args",
		"f()",
		`Call(func=Name(id="f"), args=[], keywords=[])`,
	},
	{
		"call_one_arg",
		"f(1)",
		`Call(func=Name(id="f"), args=[Constant(value=1)], keywords=[])`,
	},
	{
		"call_kw",
		"f(x=1, y=2)",
		`Call(func=Name(id="f"), args=[], keywords=[x=Constant(value=1), y=Constant(value=2)])`,
	},
	{
		"call_starred",
		"f(*xs, **kw)",
		`Call(func=Name(id="f"), args=[Starred(value=Name(id="xs"))], keywords=[**Name(id="kw")])`,
	},
	{
		"method_call",
		"obj.method(arg)",
		`Call(func=Attribute(value=Name(id="obj"), attr="method"), args=[Name(id="arg")], keywords=[])`,
	},

	// ----- Collections -----
	{"empty_tuple", "()", `Tuple([])`},
	{"empty_list", "[]", `List([])`},
	{"empty_dict", "{}", `Dict(keys=[], values=[])`},
	{
		"list_basic",
		"[1, 2, 3]",
		`List([Constant(value=1), Constant(value=2), Constant(value=3)])`,
	},
	{
		"tuple_paren",
		"(1, 2, 3)",
		`Tuple([Constant(value=1), Constant(value=2), Constant(value=3)])`,
	},
	{
		"set_basic",
		"{1, 2, 3}",
		`Set([Constant(value=1), Constant(value=2), Constant(value=3)])`,
	},
	{
		"dict_basic",
		`{"a": 1, "b": 2}`,
		`Dict(keys=[Constant(value="a"), Constant(value="b")], values=[Constant(value=1), Constant(value=2)])`,
	},
	{
		"dict_unpack",
		`{**other, "k": v}`,
		`Dict(keys=[nil, Constant(value="k")], values=[Name(id="other"), Name(id="v")])`,
	},
	{
		"list_starred",
		"[*xs, y]",
		`List([Starred(value=Name(id="xs")), Name(id="y")])`,
	},

	// ----- Comprehensions -----
	{
		"listcomp_basic",
		"[x for x in xs]",
		`ListComp(elt=Name(id="x"), for(target=Name(id="x"), iter=Name(id="xs")))`,
	},
	{
		"listcomp_with_if",
		"[x for x in xs if x > 0]",
		`ListComp(elt=Name(id="x"), for(target=Name(id="x"), iter=Name(id="xs"), ifs=[Compare(left=Name(id="x"), ops=[Gt], comparators=[Constant(value=0)])]))`,
	},
	{
		"setcomp",
		"{x for x in xs}",
		`SetComp(elt=Name(id="x"), for(target=Name(id="x"), iter=Name(id="xs")))`,
	},
	{
		"dictcomp",
		"{k: v for k, v in items}",
		`DictComp(key=Name(id="k"), value=Name(id="v"), for(target=Tuple([Name(id="k"), Name(id="v")]), iter=Name(id="items")))`,
	},
	{
		"genexp_paren",
		"(x for x in xs)",
		`GeneratorExp(elt=Name(id="x"), for(target=Name(id="x"), iter=Name(id="xs")))`,
	},

	// ----- Lambda -----
	{
		"lambda_no_args",
		"lambda: 1",
		`Lambda(args=Arguments(args=[]), body=Constant(value=1))`,
	},
	{
		"lambda_one_arg",
		"lambda x: x + 1",
		`Lambda(args=Arguments(args=[x]), body=BinOp(op=Add, left=Name(id="x"), right=Constant(value=1)))`,
	},
	{
		"lambda_default",
		"lambda x, y=2: x + y",
		`Lambda(args=Arguments(args=[x, y]), body=BinOp(op=Add, left=Name(id="x"), right=Name(id="y")))`,
	},
	{
		"lambda_starargs",
		"lambda *args, **kw: args",
		`Lambda(args=Arguments(args=[], vararg=args, kwarg=kw), body=Name(id="args"))`,
	},

	// ----- Walrus -----
	{
		"walrus_basic",
		"(x := 5)",
		`NamedExpr(target=Name(id="x"), value=Constant(value=5))`,
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

// Error cases. Each row asserts that the parser rejects the input.
var errTable = []struct {
	name    string
	src     string
	wantSub string
}{
	{"empty", "", "unexpected token"},
	{"trailing_op", "1 +", "unexpected token"},
	{"bare_op", "*", "unexpected token"},
	{"unterminated_paren", "(1 + 2", "expected"},
	{"trailing_garbage", "1 2", "unexpected token"},
	{"bad_kw_no_eq", "f(x =", "unexpected token"},
	{"unterminated_string", `"hello`, "unterminated string"},
	{"fstring_not_implemented", `f"hi"`, "f-string"},
	{"tstring_not_implemented", `t"hi"`, "t-string"},
	{"if_no_else", "a if b", "expected 'else'"},
	{"lambda_no_colon", "lambda x", "expected"},
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
// reflects 1-indexed line and 0-indexed column.
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
