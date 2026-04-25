package ast

import (
	"testing"

	"github.com/tamnd/gopapy/v1/parser"
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
	// Matches CPython 3.14 ast.dump default (show_empty=False).
	want := "Module(body=[Expr(value=BinOp(left=Constant(value=1), op=Add(), right=Constant(value=2)))])"
	if got != want {
		t.Errorf("Dump mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestDump_NameStore(t *testing.T) {
	got := dumpString(t, "x = 1\n")
	want := "Module(body=[Assign(targets=[Name(id='x', ctx=Store())], value=Constant(value=1))])"
	if got != want {
		t.Errorf("Dump mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestDump_PassStmt(t *testing.T) {
	got := dumpString(t, "pass\n")
	want := "Module(body=[Pass()])"
	if got != want {
		t.Errorf("Dump mismatch\n got: %s\nwant: %s", got, want)
	}
}
