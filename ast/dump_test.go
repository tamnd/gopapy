package ast

import (
	"testing"

	"github.com/tamnd/gopapy/parser"
)

func dumpString(t *testing.T, src string) string {
	t.Helper()
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return Dump(FromFile(f))
}

func TestDump_BinaryAdd(t *testing.T) {
	got := dumpString(t, "1 + 2\n")
	// Module(body=[Expr(value=BinOp(left=Constant(value=1, kind=None), op=Add(), right=Constant(value=2, kind=None)))], type_ignores=[])
	want := "Module(body=[Expr(value=BinOp(left=Constant(value=1, kind=None), op=Add(), right=Constant(value=2, kind=None)))], type_ignores=[])"
	if got != want {
		t.Errorf("Dump mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestDump_NameStore(t *testing.T) {
	got := dumpString(t, "x = 1\n")
	want := "Module(body=[Assign(targets=[Name(id=\"x\", ctx=Store())], value=Constant(value=1, kind=None), type_comment=None)], type_ignores=[])"
	if got != want {
		t.Errorf("Dump mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestDump_PassStmt(t *testing.T) {
	got := dumpString(t, "pass\n")
	want := "Module(body=[Pass()], type_ignores=[])"
	if got != want {
		t.Errorf("Dump mismatch\n got: %s\nwant: %s", got, want)
	}
}
