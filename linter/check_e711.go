package linter

import (
	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/diag"
)

// checkE711 fires on `==` / `!=` against `None`. PEP 8 says use `is`
// / `is not` for None — the singleton's identity is the whole point.
//
// The check fires on either side: `None == x` is just as wrong as
// `x == None`. pycodestyle catches both. The diagnostic Pos is the
// Compare node's position, matching F632's style.
func checkE711(mod *ast.Module) []diag.Diagnostic {
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
			if isNoneConstant(left) || isNoneConstant(right) {
				out = append(out, diag.Diagnostic{
					Pos:      c.Pos,
					End:      c.Pos,
					Severity: diag.SeverityWarning,
					Code:     CodeComparisonToNone,
					Msg:      "comparison to None should be `if cond is None:`",
				})
			}
			left = right
		}
	})
	return out
}

func isEqOrNotEq(op ast.CmpopNode) bool {
	switch op.(type) {
	case *ast.Eq, *ast.NotEq:
		return true
	}
	return false
}

func isNoneConstant(e ast.ExprNode) bool {
	c, ok := e.(*ast.Constant)
	return ok && c.Value.Kind == ast.ConstantNone
}
