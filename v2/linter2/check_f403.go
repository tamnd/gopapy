package linter2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

func checkF403(mod *parser2.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkAllStmts(mod, func(s parser2.Stmt) {
		imp, ok := s.(*parser2.ImportFrom)
		if !ok {
			return
		}
		for _, a := range imp.Names {
			if a.Name == "*" {
				out = append(out, diag.Diagnostic{
					Pos:      imp.P,
					End:      imp.P,
					Severity: diag.SeverityWarning,
					Code:     CodeStarImport,
					Msg:      "use of star import 'from " + imp.Module + " import *'",
				})
				break
			}
		}
	})
	return out
}

// walkAllStmts visits every Stmt in the module (including nested ones)
// by calling fn for each statement.
func walkAllStmts(mod *parser2.Module, fn func(parser2.Stmt)) {
	var walkStmts func(stmts []parser2.Stmt)
	walkStmts = func(stmts []parser2.Stmt) {
		for _, s := range stmts {
			fn(s)
			switch n := s.(type) {
			case *parser2.If:
				walkStmts(n.Body)
				walkStmts(n.Orelse)
			case *parser2.While:
				walkStmts(n.Body)
				walkStmts(n.Orelse)
			case *parser2.For:
				walkStmts(n.Body)
				walkStmts(n.Orelse)
			case *parser2.AsyncFor:
				walkStmts(n.Body)
				walkStmts(n.Orelse)
			case *parser2.With:
				walkStmts(n.Body)
			case *parser2.AsyncWith:
				walkStmts(n.Body)
			case *parser2.Try:
				walkStmts(n.Body)
				for _, h := range n.Handlers {
					walkStmts(h.Body)
				}
				walkStmts(n.Orelse)
				walkStmts(n.Finalbody)
			case *parser2.FunctionDef:
				walkStmts(n.Body)
			case *parser2.AsyncFunctionDef:
				walkStmts(n.Body)
			case *parser2.ClassDef:
				walkStmts(n.Body)
			case *parser2.Match:
				for _, c := range n.Cases {
					walkStmts(c.Body)
				}
			}
		}
	}
	walkStmts(mod.Body)
}
