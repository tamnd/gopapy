package linter2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

func checkF541(mod *parser2.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkModule(mod, func(e parser2.Expr) {
		js, ok := e.(*parser2.JoinedStr)
		if !ok {
			return
		}
		for _, v := range js.Values {
			if _, ok := v.(*parser2.FormattedValue); ok {
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
