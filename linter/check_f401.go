package linter

import (
	"github.com/tamnd/gopapy/v1/ast"
	"github.com/tamnd/gopapy/v1/diag"
	"github.com/tamnd/gopapy/v1/symbols"
)

// checkF401 fires for module-scope names bound by an import that the
// file never reads. Class-body and function-body imports are common
// for lazy-loading patterns; flagging them produces noise out of
// proportion to the value, so the check stays at module scope.
//
// Exempted shapes:
//
//   - `from M import *` binds the literal name `*`, which symbols
//     skips, so no F401 is produced for star imports.
//   - `from __future__ import X` changes parser/compiler behavior
//     regardless of whether X is referenced; the bound name is a
//     by-product. Removing it would silently change semantics, so
//     F401 ignores it.
//   - `__all__` membership is intentionally not consulted (spec).
//     Users who care can add `# noqa: F401`.
func checkF401(sm *symbols.Module, mod *ast.Module) []diag.Diagnostic {
	if sm == nil || sm.Root == nil {
		return nil
	}
	exempt := futureImportNames(mod)
	var out []diag.Diagnostic
	for name, sym := range sm.Root.Symbols {
		if !sym.Has(symbols.FlagImport) {
			continue
		}
		if sym.Has(symbols.FlagUsed) {
			continue
		}
		if exempt[name] {
			continue
		}
		if len(sym.BindSites) == 0 {
			continue
		}
		pos := sym.BindSites[0]
		out = append(out, diag.Diagnostic{
			Pos:      pos,
			End:      pos,
			Severity: diag.SeverityWarning,
			Code:     CodeUnusedImport,
			Msg:      "'" + name + "' imported but unused",
		})
	}
	return out
}

// futureImportNames collects names bound by `from __future__ import X`
// (and, defensively, `import __future__`). Returns nil for a nil mod.
func futureImportNames(mod *ast.Module) map[string]bool {
	if mod == nil {
		return nil
	}
	out := map[string]bool{}
	for _, s := range mod.Body {
		switch n := s.(type) {
		case *ast.ImportFrom:
			if n.Module != "__future__" {
				continue
			}
			for _, a := range n.Names {
				if a.Name == "*" {
					continue
				}
				name := a.Asname
				if name == "" {
					name = a.Name
				}
				out[name] = true
			}
		case *ast.Import:
			for _, a := range n.Names {
				if a.Name != "__future__" {
					continue
				}
				name := a.Asname
				if name == "" {
					name = "__future__"
				}
				out[name] = true
			}
		}
	}
	return out
}
