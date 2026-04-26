package linter2

import (
	"sort"

	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
	"github.com/tamnd/gopapy/v2/symbols2"
)

func checkF811(sm *symbols2.Module) []diag.Diagnostic {
	if sm == nil || sm.Root == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkScope(sm.Root, func(scope *symbols2.Scope) {
		for name, sym := range scope.Symbols {
			out = append(out, f811For(name, sym)...)
		}
	})
	return out
}

func f811For(name string, sym *symbols2.Binding) []diag.Diagnostic {
	if len(sym.BindSites) < 2 {
		return nil
	}
	if sym.Has(symbols2.FlagParam) || sym.Has(symbols2.FlagGlobal) || sym.Has(symbols2.FlagNonlocal) {
		return nil
	}
	binds := append([]parser2.Pos(nil), sym.BindSites...)
	uses := append([]parser2.Pos(nil), sym.UseSites...)
	sort.SliceStable(binds, func(i, j int) bool { return posBefore(binds[i], binds[j]) })
	sort.SliceStable(uses, func(i, j int) bool { return posBefore(uses[i], uses[j]) })

	var out []diag.Diagnostic
	for i := 1; i < len(binds); i++ {
		prev, cur := binds[i-1], binds[i]
		if hasUseInRange(uses, prev, cur) {
			continue
		}
		out = append(out, diag.Diagnostic{
			Pos:      cur,
			End:      cur,
			Severity: diag.SeverityWarning,
			Code:     CodeRedefinitionUnused,
			Msg:      "redefinition of unused '" + name + "'",
		})
	}
	return out
}

func hasUseInRange(uses []parser2.Pos, lo, hi parser2.Pos) bool {
	for _, u := range uses {
		if posBefore(lo, u) && !posBefore(hi, u) {
			return true
		}
	}
	return false
}

func posBefore(a, b parser2.Pos) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Col < b.Col
}

func walkScope(s *symbols2.Scope, fn func(*symbols2.Scope)) {
	if s == nil {
		return
	}
	fn(s)
	for _, c := range s.Children {
		walkScope(c, fn)
	}
}
