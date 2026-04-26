package ast

import (
	"reflect"
	"strconv"
	"testing"
)

// identityTransformer returns whatever it gets — the no-op
// transformer. Apply with this should leave the AST byte-equal.
type identityTransformer struct{}

func (identityTransformer) Transform(n Node) Node { return n }

func TestApply_IdentityIsNoOp(t *testing.T) {
	mod := parseModule(t, "x = 1 + 2\ndef f(): return x\n")
	// Capture a few stable signals from the original AST.
	var beforeNames []string
	WalkPreorder(mod, func(n Node) {
		if name, ok := n.(*Name); ok {
			beforeNames = append(beforeNames, name.Id)
		}
	})

	got := Apply(identityTransformer{}, mod)
	if got != Node(mod) {
		t.Fatalf("identity Apply returned a different root")
	}

	var afterNames []string
	WalkPreorder(mod, func(n Node) {
		if name, ok := n.(*Name); ok {
			afterNames = append(afterNames, name.Id)
		}
	})
	if !reflect.DeepEqual(beforeNames, afterNames) {
		t.Errorf("identity changed names: before=%v after=%v", beforeNames, afterNames)
	}
}

// renameTransformer rewrites every Name with id == From into a new
// Name with id == To. Position copied across so downstream tooling
// keeps source attribution.
type renameTransformer struct{ From, To string }

func (r renameTransformer) Transform(n Node) Node {
	if name, ok := n.(*Name); ok && name.Id == r.From {
		return &Name{Pos: name.Pos, Id: r.To, Ctx: name.Ctx}
	}
	return n
}

func TestApply_RenameReplacesScalarSlot(t *testing.T) {
	mod := parseModule(t, "old = old + 1\nf(old)\n")
	Apply(renameTransformer{From: "old", To: "new"}, mod)

	var ids []string
	WalkPreorder(mod, func(n Node) {
		if name, ok := n.(*Name); ok {
			ids = append(ids, name.Id)
		}
	})
	for _, id := range ids {
		if id == "old" {
			t.Errorf("found unexpected 'old' after rename: %v", ids)
		}
	}
	// All "old" references became "new"; the Assign target, the
	// right-hand-side reference, and the call argument.
	want := 4 // Module sees 4 Names: target, value-Name, call-func, call-arg
	if len(ids) != want {
		t.Errorf("expected %d Name nodes, got %d (%v)", want, len(ids), ids)
	}
}

// constFold replaces BinOp(Add, Constant(int), Constant(int)) with a
// single Constant whose value is the sum. Demonstrates a scalar-slot
// replacement under a parent (the Assign's Value).
type constFold struct{}

func (constFold) Transform(n Node) Node {
	bo, ok := n.(*BinOp)
	if !ok {
		return n
	}
	if _, isAdd := bo.Op.(*Add); !isAdd {
		return n
	}
	l, lok := bo.Left.(*Constant)
	r, rok := bo.Right.(*Constant)
	if !lok || !rok {
		return n
	}
	if l.Value.Kind != ConstantInt || r.Value.Kind != ConstantInt {
		return n
	}
	li, err1 := strconv.ParseInt(l.Value.Int, 0, 64)
	ri, err2 := strconv.ParseInt(r.Value.Int, 0, 64)
	if err1 != nil || err2 != nil {
		return n
	}
	return &Constant{
		Pos:   bo.Pos,
		Value: ConstantValue{Kind: ConstantInt, Int: strconv.FormatInt(li+ri, 10)},
	}
}

func TestApply_ConstantFoldsBinOp(t *testing.T) {
	mod := parseModule(t, "x = 1 + 2\n")
	Apply(constFold{}, mod)
	stmt := mod.Body[0].(*Assign)
	c, ok := stmt.Value.(*Constant)
	if !ok {
		t.Fatalf("expected Assign.Value to be *Constant, got %T", stmt.Value)
	}
	if c.Value.Kind != ConstantInt || c.Value.Int != "3" {
		t.Errorf("folded value = %+v, want Int=\"3\"", c.Value)
	}
}

// dropPass returns nil for every Pass statement so list-slot removal
// is exercised on a real container (function body).
type dropPass struct{}

func (dropPass) Transform(n Node) Node {
	if _, ok := n.(*Pass); ok {
		return nil
	}
	return n
}

func TestApply_NilRemovesFromList(t *testing.T) {
	mod := parseModule(t, "def f():\n    pass\n    x = 1\n    pass\n    pass\n")
	Apply(dropPass{}, mod)
	fn := mod.Body[0].(*FunctionDef)
	if got := len(fn.Body); got != 1 {
		t.Errorf("body len = %d, want 1 (only the Assign)", got)
	}
	if _, ok := fn.Body[0].(*Assign); !ok {
		t.Errorf("body[0] = %T, want *Assign", fn.Body[0])
	}
}

func TestApply_NilVisitorOrNode(t *testing.T) {
	// Both nil-transformer and nil-node must be no-ops, matching Visit.
	if got := Apply(nil, &Module{}); got == nil {
		t.Errorf("Apply(nil, mod) returned nil; want the original mod")
	}
	if got := Apply(identityTransformer{}, nil); got != nil {
		t.Errorf("Apply(t, nil) = %v; want nil", got)
	}
}

// replaceRoot returns a different Module entirely — the replacement
// must be the value Apply hands back, and the original must be left
// alone (no descend into its children).
type replaceRoot struct{ With *Module }

func (r replaceRoot) Transform(n Node) Node {
	if _, ok := n.(*Module); ok {
		return r.With
	}
	return n
}

func TestApply_ReplacesRoot(t *testing.T) {
	orig := parseModule(t, "x = 1\n")
	replacement := &Module{Body: []StmtNode{&Pass{}}}
	got := Apply(replaceRoot{With: replacement}, orig)
	if got != Node(replacement) {
		t.Fatalf("Apply returned %p, want replacement %p", got, replacement)
	}
	// The original root's body must be unchanged — no descend.
	if len(orig.Body) != 1 {
		t.Errorf("original body should be untouched, len = %d", len(orig.Body))
	}
}
