package linter

import (
	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/symbols"
)

// checkF841 fires when a local in a function or lambda body is bound
// by a plain assignment (Assign / AnnAssign with a value / NamedExpr
// walrus) and never read.
//
// The symbol table can't tell us *how* a name got bound, so we
// re-walk the AST looking only at the binding shapes the spec wants
// us to flag. Loop targets, except handlers, with-as variables, and
// pattern captures bind via different ast nodes and so don't appear
// here at all — that's how they stay exempt without a special-case
// list. Augmented assignment binds *and* reads the target so it's
// already filtered by the FlagUsed check.
func checkF841(sm *symbols.Module, mod *ast.Module) []diag.Diagnostic {
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

// walkScope walks stmts under the given symbols.Scope. inFunc says
// whether stmts live in a function or lambda body (the only place
// F841 fires).
func (c *f841Checker) walkScope(scope *symbols.Scope, stmts []ast.StmtNode, inFunc bool) {
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
		case *ast.FunctionDef:
			if child := nextChild(symbols.ScopeFunction, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, true)
				c.out = append(c.out, inner.out...)
			}
		case *ast.AsyncFunctionDef:
			if child := nextChild(symbols.ScopeFunction, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, true)
				c.out = append(c.out, inner.out...)
			}
		case *ast.ClassDef:
			if child := nextChild(symbols.ScopeClass, n.Name); child != nil {
				inner := &f841Checker{}
				inner.walkScope(child, n.Body, false)
				c.out = append(c.out, inner.out...)
			}
		case *ast.Assign:
			if inFunc && len(n.Targets) == 1 {
				c.checkTarget(scope, n.Targets[0])
			}
		case *ast.AnnAssign:
			if inFunc && n.Value != nil {
				c.checkTarget(scope, n.Target)
			}
		case *ast.If:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *ast.While:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *ast.For:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *ast.AsyncFor:
			c.walkScope(scope, n.Body, inFunc)
			c.walkScope(scope, n.Orelse, inFunc)
		case *ast.With:
			c.walkScope(scope, n.Body, inFunc)
		case *ast.AsyncWith:
			c.walkScope(scope, n.Body, inFunc)
		case *ast.Try:
			c.walkScope(scope, n.Body, inFunc)
			for _, h := range n.Handlers {
				if eh, ok := h.(*ast.ExceptHandler); ok {
					c.walkScope(scope, eh.Body, inFunc)
				}
			}
			c.walkScope(scope, n.Orelse, inFunc)
			c.walkScope(scope, n.Finalbody, inFunc)
		case *ast.TryStar:
			c.walkScope(scope, n.Body, inFunc)
			for _, h := range n.Handlers {
				if eh, ok := h.(*ast.ExceptHandler); ok {
					c.walkScope(scope, eh.Body, inFunc)
				}
			}
			c.walkScope(scope, n.Orelse, inFunc)
			c.walkScope(scope, n.Finalbody, inFunc)
		case *ast.Match:
			for _, mc := range n.Cases {
				c.walkScope(scope, mc.Body, inFunc)
			}
		}
	}
}

// checkTarget fires F841 if target is a single Name whose binding in
// scope is bound but never read and not classified as parameter,
// import, global, or nonlocal.
func (c *f841Checker) checkTarget(scope *symbols.Scope, target ast.ExprNode) {
	name, ok := target.(*ast.Name)
	if !ok {
		return
	}
	if _, isStore := name.Ctx.(*ast.Store); !isStore {
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
		Pos:      name.Pos,
		End:      name.Pos,
		Severity: diag.SeverityWarning,
		Code:     CodeUnusedLocal,
		Msg:      "local variable '" + name.Id + "' assigned but never used",
	})
}
