package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
)

func checkF631(mod *parser.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkAllStmts(mod, func(s parser.Stmt) {
		a, ok := s.(*parser.Assert)
		if !ok {
			return
		}
		tup, ok := a.Test.(*parser.Tuple)
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
