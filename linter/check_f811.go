package linter

import (
	"sort"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/symbols"
)

// checkF811 fires when a name is bound twice in the same scope and
// no read sits between the two binds. Walks every scope in the tree
// (module, function, class, lambda, comprehension) so a redefinition
// inside a method body is caught alongside one at module level.
//
// The bind/use site lists symbols.Build records preserve source
// order, but we sort defensively before pairwise comparison so a
// future change in the symbols package can't quietly break us.
func checkF811(sm *symbols.Module) []diag.Diagnostic {
	if sm == nil || sm.Root == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkScope(sm.Root, func(scope *symbols.Scope) {
		for name, sym := range scope.Symbols {
			out = append(out, f811For(scope, name, sym)...)
		}
	})
	return out
}

func f811For(scope *symbols.Scope, name string, sym *symbols.Binding) []diag.Diagnostic {
	if len(sym.BindSites) < 2 {
		return nil
	}
	// Parameters are implicitly used on function entry; pyflakes treats
	// `def f(x): x = 1` as a normal local rebind, not a redefinition.
	// global/nonlocal declarations participate in cross-scope binding,
	// not local redefinition.
	if sym.Has(symbols.FlagParam) || sym.Has(symbols.FlagGlobal) || sym.Has(symbols.FlagNonlocal) {
		return nil
	}
	binds := append([]ast.Pos(nil), sym.BindSites...)
	uses := append([]ast.Pos(nil), sym.UseSites...)
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
	_ = scope // kept for future extension (e.g. scope-name in message)
	return out
}

// hasUseInRange reports whether any use position falls in (lo, hi].
// The lower bound is strict — a load at the first bind site itself
// would just be the bind. The upper bound is inclusive because an
// augmented assign records both a bind and a use at the same
// position, and that should count: `x = 1; x += 1` is not a dead
// store. uses must be sorted.
func hasUseInRange(uses []ast.Pos, lo, hi ast.Pos) bool {
	for _, u := range uses {
		if posBefore(lo, u) && !posBefore(hi, u) {
			return true
		}
	}
	return false
}

// walkScope visits every scope in the tree in pre-order.
func walkScope(s *symbols.Scope, fn func(*symbols.Scope)) {
	if s == nil {
		return
	}
	fn(s)
	for _, c := range s.Children {
		walkScope(c, fn)
	}
}
