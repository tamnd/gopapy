package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
	"github.com/tamnd/gopapy/symbols"
)

func checkF841(sm *symbols.Module, mod *parser.Module) []diag.Diagnostic {
	if sm == nil || sm.Root == nil || mod == nil {
		return nil
	}
	c := &f841Checker{}
	c.walkScope(sm.Root, mod.Body, false)
	return c.out
}

type f841Checker struct {
	out []diag.Diagnostic
}

func (c *f841Checker) walkScope(scope *symbols.Scope, stmts []parser.Stmt, inFunc bool) {
	childIdx := 0
	nextChild := func(kind symbols.ScopeKind, name string) *symbols.Scope {
		for childIdx < len(scope.Children) {
			child := scope.Children[childIdx]
			childIdx++
			if child.Kind == kind && child.Name == name {
				return child
			}
		}
		return nil
	}
	for _, s := range stmts {
		switch n := s.(type) {
		case *parser.FunctionDef:
			if child := nextChild(symbols.ScopeFunction, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, true)
				c.out = append(c.out, inner.out...)
			}
		case *parser.AsyncFunctionDef:
			if child := nextChild(symbols.ScopeFunction, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, true)
				c.out = append(c.out, inner.out...)
			}
		case *parser.ClassDef:
			if child := nextChild(symbols.ScopeClass, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, false)
				c.out = append(c.out, inner.out...)
			}
		case *parser.Assign:
			if inFunc && len(n.Targets) == 1 {
				c.checkTarget(scope, n.Targets[0])
			}
		case *parser.AnnAssign:
			if inFunc && n.Value != nil {
				c.checkTarget(scope, n.Target)
			}
		case *parser.If:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *parser.While:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *parser.For:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *parser.AsyncFor:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *parser.With:
			c.walkScope(scope, n.Body, inFunc)
		case *parser.AsyncWith:
			c.walkScope(scope, n.Body, inFunc)
		case *parser.Try:
			c.walkScope(scope, n.Body, inFunc)
			for _, h := range n.Handlers {
				c.walkScope(scope, h.Body, inFunc)
			}
			c.walkScope(scope, n.Orelse, inFunc)
			c.walkScope(scope, n.Finalbody, inFunc)
		case *parser.Match:
			for _, mc := range n.Cases {
				c.walkScope(scope, mc.Body, inFunc)
			}
		}
	}
}

// checkTarget fires F841 if target is a single Name bound but never read.
// In v2, target Names in assignment position are always stores; no Ctx check needed.
func (c *f841Checker) checkTarget(scope *symbols.Scope, target parser.Expr) {
	name, ok := target.(*parser.Name)
	if !ok {
		return
	}
	if name.Id == "" || name.Id == "_" {
		return
	}
	sym, ok := scope.Symbols[name.Id]
	if !ok {
		return
	}
	if sym.Has(symbols.FlagUsed) {
		return
	}
	if sym.Has(symbols.FlagParam) || sym.Has(symbols.FlagImport) ||
		sym.Has(symbols.FlagGlobal) || sym.Has(symbols.FlagNonlocal) {
		return
	}
	c.out = append(c.out, diag.Diagnostic{
		Pos:      name.P,
		End:      name.P,
		Severity: diag.SeverityWarning,
		Code:     CodeUnusedLocal,
		Msg:      "local variable '" + name.Id + "' assigned but never used",
	})
}
