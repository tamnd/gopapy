package linter

import (
	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/legacy/diag"
)

// checkE712 fires on `==` / `!=` against `True` or `False`. PEP 8
// says use `is True` / `is False` when identity matters, or just
// `if x:` when truthiness is enough. Either way, `==` against a
// bool literal is never the right answer.
//
// The check fires on either side: `True == x` is just as wrong as
// `x == True`. Diagnostic Pos is the Compare node's pos to match
// E711 and F632.
func checkE712(mod *ast.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	ast.WalkPreorder(mod, func(n ast.Node) {
		c, ok := n.(*ast.Compare)
		if !ok {
			return
		}
		left := c.Left
		for i, op := range c.Ops {
			if i >= len(c.Comparators) {
				break
			}
			right := c.Comparators[i]
			if !isEqOrNotEq(op) {
				left = right
				continue
			}
			if isBoolConstant(left) || isBoolConstant(right) {
				out = append(out, diag.Diagnostic{
					Pos:      c.Pos,
					End:      c.Pos,
					Severity: diag.SeverityWarning,
					Code:     CodeComparisonToBool,
					Msg:      "comparison to True/False should be `if cond is True:` or `if cond:`",
				})
			}
			left = right
		}
	})
	return out
}

func isBoolConstant(e ast.ExprNode) bool {
	c, ok := e.(*ast.Constant)
	return ok && c.Value.Kind == ast.ConstantBool
}
