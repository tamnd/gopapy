package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
)

func checkE711(mod *parser.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkModule(mod, func(e parser.Expr) {
		c, ok := e.(*parser.Compare)
		if !ok {
			return
		}
		left := c.Left
		for i, op := range c.Ops {
			if i >= len(c.Comparators) {
				break
			}
			right := c.Comparators[i]
			if op != "Eq" && op != "NotEq" {
				left = right
				continue
			}
			if isNoneConstant(left) || isNoneConstant(right) {
				out = append(out, diag.Diagnostic{
					Pos:      c.P,
					End:      c.P,
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

func isNoneConstant(e parser.Expr) bool {
	c, ok := e.(*parser.Constant)
	return ok && c.Kind == "None"
}
