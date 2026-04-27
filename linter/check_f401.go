package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
	"github.com/tamnd/gopapy/symbols"
)

func checkF401(sm *symbols.Module, mod *parser.Module) []diag.Diagnostic {
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

func futureImportNames(mod *parser.Module) map[string]bool {
	if mod == nil {
		return nil
	}
	out := map[string]bool{}
	for _, s := range mod.Body {
		switch n := s.(type) {
		case *parser.ImportFrom:
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
		case *parser.Import:
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
