package linter2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

func checkE712(mod *parser2.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkModule(mod, func(e parser2.Expr) {
		c, ok := e.(*parser2.Compare)
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
			if isBoolConstant(left) || isBoolConstant(right) {
				out = append(out, diag.Diagnostic{
					Pos:      c.P,
					End:      c.P,
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

func isBoolConstant(e parser2.Expr) bool {
	c, ok := e.(*parser2.Constant)
	return ok && (c.Kind == "True" || c.Kind == "False")
}
