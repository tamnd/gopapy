package linter2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
	"github.com/tamnd/gopapy/v2/symbols2"
)

func checkF841(sm *symbols2.Module, mod *parser2.Module) []diag.Diagnostic {
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

func (c *f841Checker) walkScope(scope *symbols2.Scope, stmts []parser2.Stmt, inFunc bool) {
	childIdx := 0
	nextChild := func(kind symbols2.ScopeKind, name string) *symbols2.Scope {
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
		case *parser2.FunctionDef:
			if child := nextChild(symbols2.ScopeFunction, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, true)
				c.out = append(c.out, inner.out...)
			}
		case *parser2.AsyncFunctionDef:
			if child := nextChild(symbols2.ScopeFunction, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, true)
				c.out = append(c.out, inner.out...)
			}
		case *parser2.ClassDef:
			if child := nextChild(symbols2.ScopeClass, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, false)
				c.out = append(c.out, inner.out...)
			}
		case *parser2.Assign:
			if inFunc && len(n.Targets) == 1 {
				c.checkTarget(scope, n.Targets[0])
			}
		case *parser2.AnnAssign:
			if inFunc && n.Value != nil {
				c.checkTarget(scope, n.Target)
			}
		case *parser2.If:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *parser2.While:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *parser2.For:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *parser2.AsyncFor:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *parser2.With:
			c.walkScope(scope, n.Body, inFunc)
		case *parser2.AsyncWith:
			c.walkScope(scope, n.Body, inFunc)
		case *parser2.Try:
			c.walkScope(scope, n.Body, inFunc)
			for _, h := range n.Handlers {
				c.walkScope(scope, h.Body, inFunc)
			}
			c.walkScope(scope, n.Orelse, inFunc)
			c.walkScope(scope, n.Finalbody, inFunc)
		case *parser2.Match:
			for _, mc := range n.Cases {
				c.walkScope(scope, mc.Body, inFunc)
			}
		}
	}
}

// checkTarget fires F841 if target is a single Name bound but never read.
// In v2, target Names in assignment position are always stores; no Ctx check needed.
func (c *f841Checker) checkTarget(scope *symbols2.Scope, target parser2.Expr) {
	name, ok := target.(*parser2.Name)
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
	if sym.Has(symbols2.FlagUsed) {
		return
	}
	if sym.Has(symbols2.FlagParam) || sym.Has(symbols2.FlagImport) ||
		sym.Has(symbols2.FlagGlobal) || sym.Has(symbols2.FlagNonlocal) {
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
