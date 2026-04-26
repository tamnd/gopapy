package linter2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
	"github.com/tamnd/gopapy/v2/symbols2"
)

func checkF401(sm *symbols2.Module, mod *parser2.Module) []diag.Diagnostic {
	if sm == nil || sm.Root == nil {
		return nil
	}
	exempt := futureImportNames(mod)
	var out []diag.Diagnostic
	for name, sym := range sm.Root.Symbols {
		if !sym.Has(symbols2.FlagImport) {
			continue
		}
		if sym.Has(symbols2.FlagUsed) {
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

func futureImportNames(mod *parser2.Module) map[string]bool {
	if mod == nil {
		return nil
	}
	out := map[string]bool{}
	for _, s := range mod.Body {
		switch n := s.(type) {
		case *parser2.ImportFrom:
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
		case *parser2.Import:
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
