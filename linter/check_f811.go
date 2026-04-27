package linter

import (
	"sort"

	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
	"github.com/tamnd/gopapy/symbols"
)

func checkF811(sm *symbols.Module) []diag.Diagnostic {
	if sm == nil || sm.Root == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkScope(sm.Root, func(scope *symbols.Scope) {
		for name, sym := range scope.Symbols {
			out = append(out, f811For(name, sym)...)
		}
	})
	return out
}

func f811For(name string, sym *symbols.Binding) []diag.Diagnostic {
	if len(sym.BindSites) < 2 {
		return nil
	}
	if sym.Has(symbols.FlagParam) || sym.Has(symbols.FlagGlobal) || sym.Has(symbols.FlagNonlocal) {
		return nil
	}
	binds := append([]parser.Pos(nil), sym.BindSites...)
	uses := append([]parser.Pos(nil), sym.UseSites...)
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

func hasUseInRange(uses []parser.Pos, lo, hi parser.Pos) bool {
	for _, u := range uses {
		if posBefore(lo, u) && !posBefore(hi, u) {
			return true
		}
	}
	return false
}

func posBefore(a, b parser.Pos) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Col < b.Col
}

func walkScope(s *symbols.Scope, fn func(*symbols.Scope)) {
	if s == nil {
		return
	}
	fn(s)
	for _, c := range s.Children {
		walkScope(c, fn)
	}
}
