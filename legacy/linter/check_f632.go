package linter

import (
	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/legacy/diag"
)

// checkF632 fires on `is` / `is not` against a literal value:
// `x is 1`, `x is "foo"`, `x is True`, `x is (1, 2)`. CPython's
// guarantee that small ints and interned strings compare equal under
// `is` is an implementation detail, not a language promise; relying
// on it produces code that works today and breaks under PyPy or a
// future CPython. `==` is what the author meant.
//
// `is None` (and `is not None`) is the canonical idiom and stays
// silent. Container literals built from constants (`(1, 2)`,
// `frozenset({1})`) are flagged because identity comparison against a
// freshly constructed container is always False/True regardless of
// element identity.
//
// A Compare may chain: `0 is x is 1`. Each comparator is checked
// independently against its own operator slot. The diagnostic Pos is
// the comparison's pos rather than the literal's, matching pyflakes.
func checkF632(mod *ast.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	ast.WalkPreorder(mod, func(n ast.Node) {
		c, ok := n.(*ast.Compare)
		if !ok {
			return
		}
		for i, op := range c.Ops {
			if i >= len(c.Comparators) {
				break
			}
			if !isIsOp(op) {
				continue
			}
			rhs := c.Comparators[i]
			if !isLiteralForIs(rhs) {
				continue
			}
			out = append(out, diag.Diagnostic{
				Pos:      c.Pos,
				End:      c.Pos,
				Severity: diag.SeverityWarning,
				Code:     CodeIsWithLiteral,
				Msg:      "use of `is` with a literal, did you mean `==`?",
			})
		}
	})
	return out
}

func isIsOp(op ast.CmpopNode) bool {
	switch op.(type) {
	case *ast.Is, *ast.IsNot:
		return true
	}
	return false
}

// isLiteralForIs returns true when e is a literal that should never
// be compared with `is`. `None` is excluded because `is None` is the
// canonical identity check.
//
// `Ellipsis` and `NotImplemented` parse as Name (not Constant), so
// they're matched by name. The bare-dots form `...` parses as a
// Constant with kind ConstantEllipsis and goes through the Constant
// branch. `type(x) is type(y)` is intentionally not flagged: that
// shape is the canonical "same exact type" check in Python and
// pyflakes leaves it alone.
func isLiteralForIs(e ast.ExprNode) bool {
	switch v := e.(type) {
	case *ast.Constant:
		return v.Value.Kind != ast.ConstantNone
	case *ast.Tuple:
		return allConstants(v.Elts)
	case *ast.List:
		return allConstants(v.Elts)
	case *ast.Set:
		return allConstants(v.Elts)
	case *ast.Dict:
		return allConstants(v.Keys) && allConstants(v.Values)
	case *ast.UnaryOp:
		// `-1`, `+1`, `~0` parse as UnaryOp(USub, Constant(1)).
		if _, ok := v.Operand.(*ast.Constant); ok {
			switch v.Op.(type) {
			case *ast.USub, *ast.UAdd, *ast.Invert:
				return true
			}
		}
	case *ast.Name:
		// Singletons whose `is` comparison outside a dunder is
		// almost always wrong. `None` is excluded above and stays
		// the canonical identity check.
		switch v.Id {
		case "Ellipsis", "NotImplemented":
			return true
		}
	}
	return false
}

func allConstants(es []ast.ExprNode) bool {
	for _, e := range es {
		if e == nil {
			// Dict can carry a nil key for `**other` spread; treat as
			// non-literal so we don't flag `x is {**y}`.
			return false
		}
		if _, ok := e.(*ast.Constant); !ok {
			return false
		}
	}
	return true
}
