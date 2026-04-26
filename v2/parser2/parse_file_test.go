package parser2

import (
	"strings"
	"testing"
)

// TestParseFileTable runs every entry through ParseFile and compares
// DumpModule(result) against want. The dump format is parens-explicit
// and single-line so cases stay readable inline.
func TestParseFileTable(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "empty",
			src:  "",
			want: "Module(body=[])",
		},
		{
			name: "single expression statement",
			src:  "x\n",
			want: `Module(body=[Expr(value=Name(id="x"))])`,
		},
		{
			name: "missing trailing newline",
			src:  "x",
			want: `Module(body=[Expr(value=Name(id="x"))])`,
		},
		{
			name: "simple assignment",
			src:  "a = 1\n",
			want: `Module(body=[Assign(targets=[Name(id="a")], value=Constant(value=1))])`,
		},
		{
			name: "chained assignment",
			src:  "a = b = 1\n",
			want: `Module(body=[Assign(targets=[Name(id="a"), Name(id="b")], value=Constant(value=1))])`,
		},
		{
			name: "tuple unpacking",
			src:  "a, b = 1, 2\n",
			want: `Module(body=[Assign(targets=[Tuple([Name(id="a"), Name(id="b")])], value=Tuple([Constant(value=1), Constant(value=2)]))])`,
		},
		{
			name: "augmented assignment",
			src:  "x += 1\n",
			want: `Module(body=[AugAssign(target=Name(id="x"), op=Add, value=Constant(value=1))])`,
		},
		{
			name: "annotated assignment with value",
			src:  "x: int = 0\n",
			want: `Module(body=[AnnAssign(target=Name(id="x"), annotation=Name(id="int"), value=Constant(value=0), simple=true)])`,
		},
		{
			name: "annotated assignment without value",
			src:  "x: int\n",
			want: `Module(body=[AnnAssign(target=Name(id="x"), annotation=Name(id="int"), simple=true)])`,
		},
		{
			name: "pass break continue",
			src:  "pass\nbreak\ncontinue\n",
			want: `Module(body=[Pass(), Break(), Continue()])`,
		},
		{
			name: "return value",
			src:  "return 42\n",
			want: `Module(body=[Return(value=Constant(value=42))])`,
		},
		{
			name: "bare return",
			src:  "return\n",
			want: `Module(body=[Return()])`,
		},
		{
			name: "raise from",
			src:  "raise ValueError() from exc\n",
			want: `Module(body=[Raise(exc=Call(func=Name(id="ValueError"), args=[], keywords=[]), cause=Name(id="exc"))])`,
		},
		{
			name: "import simple",
			src:  "import os\n",
			want: `Module(body=[Import(names=[Alias(name="os")])])`,
		},
		{
			name: "import as",
			src:  "import os as o, sys\n",
			want: `Module(body=[Import(names=[Alias(name="os", asname="o"), Alias(name="sys")])])`,
		},
		{
			name: "from import",
			src:  "from os.path import join, exists as e\n",
			want: `Module(body=[ImportFrom(module="os.path", names=[Alias(name="join"), Alias(name="exists", asname="e")], level=0)])`,
		},
		{
			name: "from relative star",
			src:  "from . import *\n",
			want: `Module(body=[ImportFrom(module="", names=[Alias(name="*")], level=1)])`,
		},
		{
			name: "global names",
			src:  "global a, b\n",
			want: `Module(body=[Global(names=["a" "b"])])`,
		},
		{
			name: "nonlocal names",
			src:  "nonlocal x\n",
			want: `Module(body=[Nonlocal(names=["x"])])`,
		},
		{
			name: "del targets",
			src:  "del a, b[0]\n",
			want: `Module(body=[Delete(targets=[Name(id="a"), Subscript(value=Name(id="b"), slice=Constant(value=0))])])`,
		},
		{
			name: "assert with msg",
			src:  "assert x, 'oh no'\n",
			want: `Module(body=[Assert(test=Name(id="x"), msg=Constant(value="oh no"))])`,
		},
		{
			name: "if simple",
			src:  "if x:\n    a = 1\n",
			want: `Module(body=[If(test=Name(id="x"), body=[Assign(targets=[Name(id="a")], value=Constant(value=1))], orelse=[])])`,
		},
		{
			name: "if elif else",
			src:  "if x:\n    a = 1\nelif y:\n    a = 2\nelse:\n    a = 3\n",
			want: `Module(body=[If(test=Name(id="x"), body=[Assign(targets=[Name(id="a")], value=Constant(value=1))], orelse=[If(test=Name(id="y"), body=[Assign(targets=[Name(id="a")], value=Constant(value=2))], orelse=[Assign(targets=[Name(id="a")], value=Constant(value=3))])])])`,
		},
		{
			name: "while else",
			src:  "while x:\n    pass\nelse:\n    break\n",
			want: `Module(body=[While(test=Name(id="x"), body=[Pass()], orelse=[Break()])])`,
		},
		{
			name: "for loop",
			src:  "for i in xs:\n    pass\n",
			want: `Module(body=[For(target=Name(id="i"), iter=Name(id="xs"), body=[Pass()], orelse=[])])`,
		},
		{
			name: "for tuple target",
			src:  "for k, v in items:\n    pass\n",
			want: `Module(body=[For(target=Tuple([Name(id="k"), Name(id="v")]), iter=Name(id="items"), body=[Pass()], orelse=[])])`,
		},
		{
			name: "try except",
			src:  "try:\n    a\nexcept ValueError as e:\n    b\nfinally:\n    c\n",
			want: `Module(body=[Try(body=[Expr(value=Name(id="a"))], handlers=[ExceptHandler(type=Name(id="ValueError"), name="e", body=[Expr(value=Name(id="b"))])], orelse=[], finalbody=[Expr(value=Name(id="c"))])])`,
		},
		{
			name: "with one item",
			src:  "with open('f') as f:\n    pass\n",
			want: `Module(body=[With(items=[WithItem(context=Call(func=Name(id="open"), args=[Constant(value="f")], keywords=[]), vars=Name(id="f"))], body=[Pass()])])`,
		},
		{
			name: "with multiple items",
			src:  "with a, b as c:\n    pass\n",
			want: `Module(body=[With(items=[WithItem(context=Name(id="a")), WithItem(context=Name(id="b"), vars=Name(id="c"))], body=[Pass()])])`,
		},
		{
			name: "function def",
			src:  "def f(x, y=1):\n    return x + y\n",
			want: `Module(body=[FunctionDef(name="f", args=Arguments(args=[x, y]), body=[Return(value=BinOp(op=Add, left=Name(id="x"), right=Name(id="y")))], decorators=[], returns=nil)])`,
		},
		{
			name: "function def with annotations and return",
			src:  "def f(x: int, *args, **kw) -> str:\n    pass\n",
			want: `Module(body=[FunctionDef(name="f", args=Arguments(args=[x], vararg=args, kwarg=kw), body=[Pass()], decorators=[], returns=Name(id="str"))])`,
		},
		{
			name: "decorated function",
			src:  "@cache\ndef f():\n    pass\n",
			want: `Module(body=[FunctionDef(name="f", args=Arguments(args=[]), body=[Pass()], decorators=[Name(id="cache")], returns=nil)])`,
		},
		{
			name: "async function",
			src:  "async def f():\n    pass\n",
			want: `Module(body=[AsyncFunctionDef(name="f", args=Arguments(args=[]), body=[Pass()], decorators=[], returns=nil)])`,
		},
		{
			name: "class with bases and methods",
			src:  "class C(Base):\n    x = 1\n    def m(self):\n        return self.x\n",
			want: `Module(body=[ClassDef(name="C", bases=[Name(id="Base")], keywords=[], body=[Assign(targets=[Name(id="x")], value=Constant(value=1)), FunctionDef(name="m", args=Arguments(args=[self]), body=[Return(value=Attribute(value=Name(id="self"), attr="x"))], decorators=[], returns=nil)], decorators=[])])`,
		},
		{
			name: "class with metaclass kwarg",
			src:  "class C(Base, metaclass=Meta):\n    pass\n",
			want: `Module(body=[ClassDef(name="C", bases=[Name(id="Base")], keywords=[Keyword(arg="metaclass", value=Name(id="Meta"))], body=[Pass()], decorators=[])])`,
		},
		{
			name: "semicolon-separated simple statements",
			src:  "a = 1; b = 2; c = 3\n",
			want: `Module(body=[Assign(targets=[Name(id="a")], value=Constant(value=1)), Assign(targets=[Name(id="b")], value=Constant(value=2)), Assign(targets=[Name(id="c")], value=Constant(value=3))])`,
		},
		{
			name: "nested blocks",
			src:  "if a:\n    if b:\n        c\n    d\ne\n",
			want: `Module(body=[If(test=Name(id="a"), body=[If(test=Name(id="b"), body=[Expr(value=Name(id="c"))], orelse=[]), Expr(value=Name(id="d"))], orelse=[]), Expr(value=Name(id="e"))])`,
		},
		{
			name: "blank and comment lines",
			src:  "# top comment\n\nx = 1\n\n# trailing\n",
			want: `Module(body=[Assign(targets=[Name(id="x")], value=Constant(value=1))])`,
		},
		{
			name: "fstring no interp",
			src:  `f"hello"` + "\n",
			want: `Module(body=[Expr(value=JoinedStr(values=[Constant(value="hello")]))])`,
		},
		{
			name: "fstring single interp",
			src:  `x = f"hi {name}"` + "\n",
			want: `Module(body=[Assign(targets=[Name(id="x")], value=JoinedStr(values=[Constant(value="hi "), FormattedValue(value=Name(id="name"))]))])`,
		},
		{
			name: "fstring conversion",
			src:  `f"{x!r}"` + "\n",
			want: `Module(body=[Expr(value=JoinedStr(values=[FormattedValue(value=Name(id="x"), conversion=114)]))])`,
		},
		{
			name: "fstring format spec",
			src:  `f"{x:>10}"` + "\n",
			want: `Module(body=[Expr(value=JoinedStr(values=[FormattedValue(value=Name(id="x"), format_spec=JoinedStr(values=[Constant(value=">10")]))]))])`,
		},
		{
			name: "fstring escaped braces",
			src:  `f"{{ {x} }}"` + "\n",
			want: `Module(body=[Expr(value=JoinedStr(values=[Constant(value="{ "), FormattedValue(value=Name(id="x")), Constant(value=" }")]))])`,
		},
		{
			name: "tstring single interp",
			src:  `t"hi {name}"` + "\n",
			want: `Module(body=[Expr(value=TemplateStr(strings=[Constant(value="hi "), Constant(value="")], interpolations=[Interpolation(value=Name(id="name"), str="name")]))])`,
		},
		{
			name: "adjacent plain and fstring",
			src:  `s = "a" f"b{x}c"` + "\n",
			want: `Module(body=[Assign(targets=[Name(id="s")], value=JoinedStr(values=[Constant(value="a"), Constant(value="b"), FormattedValue(value=Name(id="x")), Constant(value="c")]))])`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mod, err := ParseFile("test.py", tc.src)
			if err != nil {
				t.Fatalf("ParseFile error: %v\nsrc=%q", err, tc.src)
			}
			got := DumpModule(mod)
			if got != tc.want {
				t.Errorf("dump mismatch:\n got: %s\nwant: %s\n src: %q", got, tc.want, tc.src)
			}
		})
	}
}

func TestParseFileErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"unindented after if", "if x:\npass\n", "expected indented block"},
		{"missing colon", "if x\n    pass\n", "expected :"},
		{"assign to literal", "1 = x\n", "cannot assign to literal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFile("test.py", tc.src)
			if err == nil {
				t.Fatalf("ParseFile(%q) succeeded; expected error containing %q", tc.src, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}
