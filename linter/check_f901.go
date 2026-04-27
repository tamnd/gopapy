package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
)

func checkF901(mod *parser.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkAllStmts(mod, func(s parser.Stmt) {
		r, ok := s.(*parser.Raise)
		if !ok || r.Exc == nil {
			return
		}
		name, ok := r.Exc.(*parser.Name)
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
