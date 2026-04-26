package ast

import (
	"testing"

	"github.com/tamnd/gopapy/v1/parser"
)

// parseAST is a helper that runs the full pipeline (parser → AST emitter)
// and returns the Module body. Tests that only care about the AST shape
// pull from this rather than re-running ParseString themselves.
func parseAST(t *testing.T, src string) []StmtNode {
	t.Helper()
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return FromFile(f).Body
}

func TestEmit_PassExpression(t *testing.T) {
	body := parseAST(t, "pass\n")
	if len(body) != 1 {
		t.Fatalf("len(body) = %d", len(body))
	}
	if _, ok := body[0].(*Pass); !ok {
		t.Errorf("body[0] = %T, want *Pass", body[0])
	}
}

func TestEmit_BinOpsLeftAssoc(t *testing.T) {
	// 1 + 2 + 3 should fold to BinOp(BinOp(1, +, 2), +, 3) — left-leaning.
	body := parseAST(t, "1 + 2 + 3\n")
	expr, ok := body[0].(*Expr)
	if !ok {
		t.Fatalf("body[0] = %T, want *Expr", body[0])
	}
	outer, ok := expr.Value.(*BinOp)
	if !ok {
		t.Fatalf("Value = %T, want *BinOp", expr.Value)
	}
	if _, ok := outer.Left.(*BinOp); !ok {
		t.Errorf("Left = %T, want *BinOp (left-leaning)", outer.Left)
	}
	if _, ok := outer.Right.(*Constant); !ok {
		t.Errorf("Right = %T, want *Constant", outer.Right)
	}
}

func TestEmit_PowerRightAssoc(t *testing.T) {
	// 2 ** 3 ** 4 should fold to BinOp(2, **, BinOp(3, **, 4)) — right-leaning.
	body := parseAST(t, "2 ** 3 ** 4\n")
	expr := body[0].(*Expr)
	outer := expr.Value.(*BinOp)
	if _, ok := outer.Left.(*Constant); !ok {
		t.Errorf("Left = %T, want *Constant (right-leaning power)", outer.Left)
	}
	if _, ok := outer.Right.(*BinOp); !ok {
		t.Errorf("Right = %T, want *BinOp (right-leaning power)", outer.Right)
	}
}

func TestEmit_CompareNotIn(t *testing.T) {
	body := parseAST(t, "a not in b\n")
	expr := body[0].(*Expr)
	cmp, ok := expr.Value.(*Compare)
	if !ok {
		t.Fatalf("Value = %T, want *Compare", expr.Value)
	}
	if len(cmp.Ops) != 1 {
		t.Fatalf("len(Ops) = %d", len(cmp.Ops))
	}
	if _, ok := cmp.Ops[0].(*NotIn); !ok {
		t.Errorf("Ops[0] = %T, want *NotIn", cmp.Ops[0])
	}
}

func TestEmit_AssignTarget(t *testing.T) {
	body := parseAST(t, "x = 1\n")
	a, ok := body[0].(*Assign)
	if !ok {
		t.Fatalf("body[0] = %T, want *Assign", body[0])
	}
	if len(a.Targets) != 1 {
		t.Fatalf("len(Targets) = %d", len(a.Targets))
	}
	name, ok := a.Targets[0].(*Name)
	if !ok {
		t.Fatalf("Targets[0] = %T, want *Name", a.Targets[0])
	}
	if name.Id != "x" {
		t.Errorf("Name.Id = %q", name.Id)
	}
	if _, ok := name.Ctx.(*Store); !ok {
		t.Errorf("Name.Ctx = %T, want *Store", name.Ctx)
	}
}

func TestEmit_If_Elif_Else(t *testing.T) {
	src := `if a:
    x
elif b:
    y
else:
    z
`
	body := parseAST(t, src)
	root, ok := body[0].(*If)
	if !ok {
		t.Fatalf("body[0] = %T, want *If", body[0])
	}
	if len(root.Orelse) != 1 {
		t.Fatalf("Orelse len = %d, want 1 (elif)", len(root.Orelse))
	}
	elif, ok := root.Orelse[0].(*If)
	if !ok {
		t.Fatalf("Orelse[0] = %T, want *If (elif chain)", root.Orelse[0])
	}
	if len(elif.Orelse) == 0 {
		t.Fatalf("elif.Orelse should contain the final else block")
	}
}

func TestEmit_Subscript_Slice(t *testing.T) {
	body := parseAST(t, "a[::2]\n")
	expr := body[0].(*Expr)
	sub, ok := expr.Value.(*Subscript)
	if !ok {
		t.Fatalf("Value = %T, want *Subscript", expr.Value)
	}
	sl, ok := sub.Slice.(*Slice)
	if !ok {
		t.Fatalf("Slice = %T, want *Slice", sub.Slice)
	}
	if sl.Lower != nil || sl.Upper != nil {
		t.Errorf("Lower/Upper should be nil for [::2]")
	}
	if sl.Step == nil {
		t.Errorf("Step should be set for [::2]")
	}
}

func TestEmit_Call_KwAndStar(t *testing.T) {
	body := parseAST(t, "f(1, x=2, *args, **kwargs)\n")
	expr := body[0].(*Expr)
	call, ok := expr.Value.(*Call)
	if !ok {
		t.Fatalf("Value = %T, want *Call", expr.Value)
	}
	// Args should contain `1` and `*args` (as Starred).
	if len(call.Args) != 2 {
		t.Errorf("len(Args) = %d, want 2", len(call.Args))
	}
	if _, ok := call.Args[1].(*Starred); !ok {
		t.Errorf("Args[1] = %T, want *Starred", call.Args[1])
	}
	// Keywords should contain x=2 and the **kwargs (Arg=="").
	if len(call.Keywords) != 2 {
		t.Errorf("len(Keywords) = %d, want 2", len(call.Keywords))
	}
	if call.Keywords[0].Arg != "x" {
		t.Errorf("Keywords[0].Arg = %q, want x", call.Keywords[0].Arg)
	}
	if call.Keywords[1].Arg != "" {
		t.Errorf("Keywords[1].Arg = %q, want \"\" (**kwargs)", call.Keywords[1].Arg)
	}
}

func TestEmit_FromRelativeImport(t *testing.T) {
	body := parseAST(t, "from .. import x\n")
	imp, ok := body[0].(*ImportFrom)
	if !ok {
		t.Fatalf("body[0] = %T, want *ImportFrom", body[0])
	}
	if imp.Level != 2 {
		t.Errorf("Level = %d, want 2", imp.Level)
	}
	if imp.Module != "" {
		t.Errorf("Module = %q, want empty", imp.Module)
	}
	if len(imp.Names) != 1 || imp.Names[0].Name != "x" {
		t.Errorf("Names = %+v", imp.Names)
	}
}

func TestEmit_DictAndSet(t *testing.T) {
	dictBody := parseAST(t, "{1: 2, 3: 4}\n")
	d, ok := dictBody[0].(*Expr).Value.(*Dict)
	if !ok {
		t.Fatalf("dict literal not emitted as *Dict")
	}
	if len(d.Keys) != 2 || len(d.Values) != 2 {
		t.Errorf("Dict keys/values = %d/%d", len(d.Keys), len(d.Values))
	}

	setBody := parseAST(t, "{1, 2, 3}\n")
	s, ok := setBody[0].(*Expr).Value.(*Set)
	if !ok {
		t.Fatalf("set literal not emitted as *Set")
	}
	if len(s.Elts) != 3 {
		t.Errorf("Set elts = %d", len(s.Elts))
	}
}

// TestEmit_DictSetMalformedDoesNotPanic covers a class of malformed-Python
// inputs the participle grammar lets through: mixing set elements with
// dict-style `**y` unpacking. CPython rejects these at parse time but
// the bootstrap grammar doesn't, so the emitter has to handle them
// without dereferencing a nil Key. The fuzz pass on PR #19 surfaced
// `{x, **y, z}` and friends as nil-pointer panics; this test pins the
// fix.
func TestEmit_DictSetMalformedDoesNotPanic(t *testing.T) {
	cases := []string{
		"{x, **y}",
		"{x, **y, z}",
		"{*x, **y}",
		"{x, *y, **z}",
		"a = {x, **y}",
		"({x, **y})",
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on %q: %v", src, r)
				}
			}()
			f, err := parser.ParseString("<test>", src)
			if err != nil {
				// The grammar rejecting it is fine — we only care that
				// the *emitter* doesn't crash on the shapes it does
				// accept.
				return
			}
			_ = FromFile(f)
		})
	}
}
