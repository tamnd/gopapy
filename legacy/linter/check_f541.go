package linter

import (
	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/legacy/diag"
)

// checkF541 fires on f-strings whose values contain no placeholder.
// `f"hello"` is identical in meaning to `"hello"` and the `f` prefix
// just adds noise plus a tiny runtime cost. The check looks at every
// JoinedStr in the module: if none of its values is a FormattedValue
// or Interpolation, the placeholder count is zero.
//
// An empty JoinedStr (Values nil) counts too — `f""` is just as
// pointless as `f"hi"`. Position is the JoinedStr's own position so
// editors highlight the whole literal.
//
// TemplateStr (PEP 750 t-strings) is intentionally not flagged. The
// `t` prefix has runtime semantics independent of placeholder count.
func checkF541(mod *ast.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	ast.WalkPreorder(mod, func(n ast.Node) {
		js, ok := n.(*ast.JoinedStr)
		if !ok {
			return
		}
		if hasInterpolation(js) {
			return
		}
		out = append(out, diag.Diagnostic{
			Pos:      js.Pos,
			End:      js.Pos,
			Severity: diag.SeverityWarning,
			Code:     CodeFStringWithoutPlaceholders,
			Msg:      "f-string without any placeholders",
		})
	})
	return out
}

func hasInterpolation(js *ast.JoinedStr) bool {
	for _, v := range js.Values {
		switch v.(type) {
		case *ast.FormattedValue, *ast.Interpolation:
			return true
		}
	}
	return false
}
