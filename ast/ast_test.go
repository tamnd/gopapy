package ast

import "testing"

// TestGenerated_ShapeMatchesAST verifies the generator produced the structs
// we expect, in the order CPython's _fields tuple uses. If this test starts
// failing after an ASDL bump, the generator output drifted from upstream.
func TestGenerated_ShapeMatchesAST(t *testing.T) {
	t.Run("BinOp", func(t *testing.T) {
		bo := &BinOp{Pos: Pos{Lineno: 3, ColOffset: 5}}
		var _ ExprNode = bo
		info := LookupNodeInfo("BinOp")
		if info == nil {
			t.Fatal("LookupNodeInfo(BinOp) = nil")
		}
		want := []string{"left", "op", "right"}
		got := make([]string, len(info.Fields))
		for i, f := range info.Fields {
			got[i] = f.Name
		}
		if !equal(got, want) {
			t.Errorf("BinOp fields = %v, want %v", got, want)
		}
		if len(info.Attributes) != 4 {
			t.Errorf("BinOp attributes = %d, want 4 (lineno/col_offset/end_lineno/end_col_offset)", len(info.Attributes))
		}
	})

	t.Run("FunctionDef order", func(t *testing.T) {
		info := LookupNodeInfo("FunctionDef")
		if info == nil {
			t.Fatal("LookupNodeInfo(FunctionDef) = nil")
		}
		want := []string{"name", "args", "body", "decorator_list", "returns", "type_comment", "type_params"}
		got := make([]string, len(info.Fields))
		for i, f := range info.Fields {
			got[i] = f.Name
		}
		if !equal(got, want) {
			t.Errorf("FunctionDef fields = %v, want %v", got, want)
		}
	})

	t.Run("Pass has no fields", func(t *testing.T) {
		info := LookupNodeInfo("Pass")
		if info == nil {
			t.Fatal("LookupNodeInfo(Pass) = nil")
		}
		if len(info.Fields) != 0 {
			t.Errorf("Pass fields = %d, want 0", len(info.Fields))
		}
		var _ StmtNode = (*Pass)(nil)
	})

	t.Run("Mod operator vs ModNode", func(t *testing.T) {
		// `Mod` is the modulo operator constructor (sum operator).
		// `ModNode` is the interface for mod (Module/Interactive/...).
		var _ OperatorNode = (*Mod)(nil)
		var _ ModNode = (*Module)(nil)
		var _ ModNode = (*Interactive)(nil)
	})

	t.Run("Expr stmt vs ExprNode", func(t *testing.T) {
		// Expr is the expression-statement (sum stmt). ExprNode is the
		// interface for the expression sum type.
		var _ StmtNode = (*Expr)(nil)
		var _ ExprNode = (*BinOp)(nil)
	})

	t.Run("Dict.keys is OptSeq", func(t *testing.T) {
		info := LookupNodeInfo("Dict")
		if info.Fields[0].Kind != FieldOptSeq {
			t.Errorf("Dict.keys kind = %v, want FieldOptSeq", info.Fields[0].Kind)
		}
	})

	t.Run("MatchSingleton uses ConstantValue", func(t *testing.T) {
		ms := &MatchSingleton{Value: ConstantValue{Kind: ConstantNone}}
		if ms.Value.Kind != ConstantNone {
			t.Error("MatchSingleton.Value should be a ConstantValue")
		}
	})
}

func TestWalk_BinOp(t *testing.T) {
	tree := &BinOp{
		Left:  &Constant{Value: ConstantValue{Kind: ConstantInt, Int: "1"}},
		Op:    &Add{},
		Right: &Constant{Value: ConstantValue{Kind: ConstantInt, Int: "2"}},
	}
	var seen []string
	Walk(tree, func(n Node) bool {
		switch n.(type) {
		case *BinOp:
			seen = append(seen, "BinOp")
		case *Constant:
			seen = append(seen, "Constant")
		case *Add:
			seen = append(seen, "Add")
		}
		return true
	})
	want := []string{"BinOp", "Constant", "Add", "Constant"}
	if !equal(seen, want) {
		t.Errorf("Walk order = %v, want %v", seen, want)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
