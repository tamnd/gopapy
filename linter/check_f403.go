package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
)

func checkF403(mod *parser.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkAllStmts(mod, func(s parser.Stmt) {
		imp, ok := s.(*parser.ImportFrom)
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
func walkAllStmts(mod *parser.Module, fn func(parser.Stmt)) {
	var walkStmts func(stmts []parser.Stmt)
	walkStmts = func(stmts []parser.Stmt) {
		for _, s := range stmts {
			fn(s)
			switch n := s.(type) {
			case *parser.If:
				walkStmts(n.Body)
				walkStmts(n.Orelse)
			case *parser.While:
				walkStmts(n.Body)
				walkStmts(n.Orelse)
			case *parser.For:
				walkStmts(n.Body)
				walkStmts(n.Orelse)
			case *parser.AsyncFor:
				walkStmts(n.Body)
				walkStmts(n.Orelse)
			case *parser.With:
				walkStmts(n.Body)
			case *parser.AsyncWith:
				walkStmts(n.Body)
			case *parser.Try:
				walkStmts(n.Body)
				for _, h := range n.Handlers {
					walkStmts(h.Body)
				}
				walkStmts(n.Orelse)
				walkStmts(n.Finalbody)
			case *parser.FunctionDef:
				walkStmts(n.Body)
			case *parser.AsyncFunctionDef:
				walkStmts(n.Body)
			case *parser.ClassDef:
				walkStmts(n.Body)
			case *parser.Match:
				for _, c := range n.Cases {
					walkStmts(c.Body)
				}
			}
		}
	}
	walkStmts(mod.Body)
}
