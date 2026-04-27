package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
)

func checkF541(mod *parser.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkModule(mod, func(e parser.Expr) {
		js, ok := e.(*parser.JoinedStr)
		if !ok {
			return
		}
		for _, v := range js.Values {
			if _, ok := v.(*parser.FormattedValue); ok {
				return
			}
		}
		out = append(out, diag.Diagnostic{
			Pos:      js.P,
			End:      js.P,
			Severity: diag.SeverityWarning,
			Code:     CodeFStringWithoutPlaceholders,
			Msg:      "f-string without any placeholders",
		})
	})
	return out
}
