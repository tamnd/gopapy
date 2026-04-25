package asdlgen

import (
	_ "embed"
	"testing"
)

//go:embed Python.asdl
var pythonASDL string

func TestParse_Python_asdl(t *testing.T) {
	m, err := Parse(pythonASDL)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Name != "Python" {
		t.Errorf("module name = %q, want Python", m.Name)
	}
	// Spot-check the eight top-level defs we know are there.
	want := []string{
		"mod", "stmt", "expr",
		"expr_context", "boolop", "operator", "unaryop", "cmpop",
		"comprehension", "excepthandler",
		"arguments", "arg", "keyword", "alias",
		"withitem", "match_case", "pattern",
		"type_ignore", "type_param",
	}
	got := map[string]*Def{}
	for _, d := range m.Defs {
		got[d.Name] = d
	}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("missing def %q", name)
		}
	}

	// expr.BinOp must have exactly three fields: left, op, right.
	expr := got["expr"]
	if expr == nil {
		t.Fatal("no expr def")
	}
	var binop *Constructor
	for _, c := range expr.Constructors {
		if c.Name == "BinOp" {
			binop = c
		}
	}
	if binop == nil {
		t.Fatal("expr.BinOp not found")
	}
	if len(binop.Fields) != 3 {
		t.Fatalf("BinOp fields = %d, want 3", len(binop.Fields))
	}
	if binop.Fields[0].Type != "expr" || binop.Fields[0].Name != "left" {
		t.Errorf("BinOp[0] = %v, want expr left", binop.Fields[0])
	}
	if binop.Fields[1].Type != "operator" || binop.Fields[1].Name != "op" {
		t.Errorf("BinOp[1] = %v, want operator op", binop.Fields[1])
	}
	if binop.Fields[2].Type != "expr" || binop.Fields[2].Name != "right" {
		t.Errorf("BinOp[2] = %v, want expr right", binop.Fields[2])
	}

	// stmt has shared attributes (lineno, col_offset, end_lineno?, end_col_offset?).
	stmt := got["stmt"]
	if len(stmt.Attributes) != 4 {
		t.Fatalf("stmt attributes = %d, want 4", len(stmt.Attributes))
	}
	if !stmt.Attributes[2].Opt {
		t.Errorf("stmt.end_lineno should be optional")
	}

	// expr.Dict has the rare expr?* (list of optional) for keys.
	var dict *Constructor
	for _, c := range expr.Constructors {
		if c.Name == "Dict" {
			dict = c
		}
	}
	if dict == nil {
		t.Fatal("expr.Dict not found")
	}
	if !dict.Fields[0].OptSeq {
		t.Errorf("Dict.keys should be expr?* (optional sequence), got %v", dict.Fields[0])
	}

	// arguments is a product type (no constructors, just fields).
	args := got["arguments"]
	if !args.IsProduct {
		t.Errorf("arguments should be a product type")
	}
	if len(args.Fields) != 7 {
		t.Errorf("arguments fields = %d, want 7", len(args.Fields))
	}

	// Pass / Break / Continue are zero-field constructors of stmt.
	stmtCtors := map[string]*Constructor{}
	for _, c := range got["stmt"].Constructors {
		stmtCtors[c.Name] = c
	}
	for _, name := range []string{"Pass", "Break", "Continue"} {
		c, ok := stmtCtors[name]
		if !ok {
			t.Errorf("stmt.%s missing", name)
			continue
		}
		if len(c.Fields) != 0 {
			t.Errorf("stmt.%s should have no fields, got %d", name, len(c.Fields))
		}
	}
}

func TestParse_Smoke(t *testing.T) {
	src := `module Tiny {
        foo = (int x, string y)
        bar = One(int a) | Two | Three(int b, int c)
              attributes (int lineno)
    }`
	m, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Defs) != 2 {
		t.Fatalf("defs = %d", len(m.Defs))
	}
	if !m.Defs[0].IsProduct || len(m.Defs[0].Fields) != 2 {
		t.Errorf("foo = %+v", m.Defs[0])
	}
	if m.Defs[1].IsProduct || len(m.Defs[1].Constructors) != 3 {
		t.Errorf("bar constructors = %d", len(m.Defs[1].Constructors))
	}
	if len(m.Defs[1].Attributes) != 1 {
		t.Errorf("bar attributes = %d", len(m.Defs[1].Attributes))
	}
}
