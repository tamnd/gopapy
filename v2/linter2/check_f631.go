package linter2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

func checkF631(mod *parser2.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkAllStmts(mod, func(s parser2.Stmt) {
		a, ok := s.(*parser2.Assert)
		if !ok {
			return
		}
		tup, ok := a.Test.(*parser2.Tuple)
		if !ok || len(tup.Elts) == 0 {
			return
		}
		out = append(out, diag.Diagnostic{
			Pos:      a.P,
			End:      a.P,
			Severity: diag.SeverityWarning,
			Code:     CodeAssertTuple,
			Msg:      "assertion is always true, perhaps remove parentheses?",
		})
	})
	return out
}
