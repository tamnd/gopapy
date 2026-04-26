package linter

import (
	"github.com/tamnd/gopapy/v1/diag"
	"github.com/tamnd/gopapy/v1/symbols"
)

// checkF401 fires for module-scope names bound by an import that the
// file never reads. Class-body and function-body imports are common
// for lazy-loading patterns; flagging them produces noise out of
// proportion to the value, so v0.1.13 stays at module scope.
//
// Suppression notes (handled in linter.go via # noqa, not here):
//   - `from M import *` binds the literal name `*` which we skip
//     in the symbols package, so no F401 is produced for star imports.
//   - `__all__` membership is intentionally not consulted in v0.1.13
//     (see spec). Users who care can add `# noqa: F401`.
func checkF401(sm *symbols.Module) []diag.Diagnostic {
	if sm == nil || sm.Root == nil {
		return nil
	}
	var out []diag.Diagnostic
	for name, sym := range sm.Root.Symbols {
		if !sym.Has(symbols.FlagImport) {
			continue
		}
		if sym.Has(symbols.FlagUsed) {
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
