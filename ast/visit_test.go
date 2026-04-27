package ast

import (
	"reflect"
	"testing"

	"github.com/tamnd/gopapy/parser"
)

// parseModule is a small helper used only in this file to keep the
// test bodies readable. Tests that need extra assertions on parse
// output go through parseAST in emit_test.go instead.
func parseModule(t *testing.T, src string) *Module {
	t.Helper()
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return FromFile(f)
}

// nameCollector is the canonical "do something per node type"
// visitor: it captures every FunctionDef name as it walks. It always
// returns itself, which is the "keep walking with me" form.
type nameCollector struct {
	names []string
}

func (c *nameCollector) Visit(n Node) Visitor {
	if fd, ok := n.(*FunctionDef); ok {
		c.names = append(c.names, fd.Name)
	}
	return c
}

func TestVisit_CollectsFunctionNames(t *testing.T) {
	mod := parseModule(t, `
def outer():
    def inner():
        pass
def sibling():
    pass
`)
	c := &nameCollector{}
	Visit(c, mod)
	want := []string{"outer", "inner", "sibling"}
	if !reflect.DeepEqual(c.names, want) {
		t.Errorf("names = %v, want %v", c.names, want)
	}
}

// pruneClassBody returns nil for ClassDef, which should stop the walk
// from entering the class's children.
type pruneClassBody struct {
	visited []string
}

func (p *pruneClassBody) Visit(n Node) Visitor {
	switch x := n.(type) {
	case *ClassDef:
		p.visited = append(p.visited, "class:"+x.Name)
		return nil
	case *FunctionDef:
		p.visited = append(p.visited, "func:"+x.Name)
	}
	return p
}

func TestVisit_NilPrunesSubtree(t *testing.T) {
	mod := parseModule(t, `
def kept():
    pass
class C:
    def hidden(self):
        pass
def alsokept():
    pass
`)
	p := &pruneClassBody{}
	Visit(p, mod)
	want := []string{"func:kept", "class:C", "func:alsokept"}
	if !reflect.DeepEqual(p.visited, want) {
		t.Errorf("visited = %v, want %v", p.visited, want)
	}
}

// swapVisitor returns a different visitor when it enters a
// FunctionDef, exercising the third return mode (different visitor for
// subtree). The inner visitor records every Name node it sees; the
// outer never does.
type outerCounter struct{ enters int }
type innerCounter struct{ names []string }

func (o *outerCounter) Visit(n Node) Visitor {
	if fd, ok := n.(*FunctionDef); ok {
		o.enters++
		_ = fd
		return &innerCounter{}
	}
	return o
}
func (i *innerCounter) Visit(n Node) Visitor {
	if name, ok := n.(*Name); ok {
		i.names = append(i.names, name.Id)
	}
	return i
}

func TestVisit_SwapsVisitorForSubtree(t *testing.T) {
	mod := parseModule(t, `
x = 1
def f():
    a
    b
y = 2
`)
	o := &outerCounter{}
	Visit(o, mod)
	if o.enters != 1 {
		t.Errorf("FunctionDef enters = %d, want 1", o.enters)
	}
}

func TestWalkPreorder_VisitsAllNodes(t *testing.T) {
	mod := parseModule(t, "x = 1 + 2\n")
	var seen []string
	WalkPreorder(mod, func(n Node) {
		seen = append(seen, kindOf(n))
	})
	// Pre-order: parent before children. The first hit is the Module
	// root, then the Assign, then its target Name and value BinOp / etc.
	if len(seen) == 0 || seen[0] != "Module" {
		t.Fatalf("expected Module first, got %v", seen)
	}
	if !contains(seen, "Assign") || !contains(seen, "BinOp") {
		t.Errorf("missing Assign or BinOp: %v", seen)
	}
}

func TestWalkPostorder_ParentAfterChildren(t *testing.T) {
	mod := parseModule(t, "x = 1\n")
	var seen []string
	WalkPostorder(mod, func(n Node) {
		seen = append(seen, kindOf(n))
	})
	// Post-order: the Module root must be last.
	if len(seen) == 0 || seen[len(seen)-1] != "Module" {
		t.Fatalf("expected Module last, got %v", seen)
	}
	// And the Assign comes after its constituent Name/Constant.
	assignIdx := indexOf(seen, "Assign")
	nameIdx := indexOf(seen, "Name")
	constIdx := indexOf(seen, "Constant")
	if assignIdx < 0 || nameIdx < 0 || constIdx < 0 {
		t.Fatalf("missing expected kinds in %v", seen)
	}
	if assignIdx < nameIdx || assignIdx < constIdx {
		t.Errorf("Assign (%d) should follow Name (%d) and Constant (%d): %v",
			assignIdx, nameIdx, constIdx, seen)
	}
}

func TestVisit_NilVisitorOrNode(t *testing.T) {
	// Both nil-visitor and nil-node must be no-ops, matching Walk.
	Visit(nil, &Module{})
	Visit(&nameCollector{}, nil)
}

func kindOf(n Node) string {
	t := reflect.TypeOf(n)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func indexOf(ss []string, s string) int {
	for i, x := range ss {
		if x == s {
			return i
		}
	}
	return -1
}
