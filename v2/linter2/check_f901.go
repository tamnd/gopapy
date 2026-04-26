package linter2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

func checkF901(mod *parser2.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkAllStmts(mod, func(s parser2.Stmt) {
		r, ok := s.(*parser2.Raise)
		if !ok || r.Exc == nil {
			return
		}
		name, ok := r.Exc.(*parser2.Name)
		if !ok {
			return
		}
		if name.Id == "NotImplemented" {
			out = append(out, diag.Diagnostic{
				Pos:      r.P,
				End:      r.P,
				Severity: diag.SeverityWarning,
				Code:     CodeRaiseNotImplemented,
				Msg:      "'raise NotImplemented' should be 'raise NotImplementedError'",
			})
		}
	})
	return out
}
