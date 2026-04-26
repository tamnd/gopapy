package linter

import (
	"github.com/tamnd/gopapy/v1/ast"
	"github.com/tamnd/gopapy/v1/symbols"
)

// FixedDiagnostic is what Fix removed: code, position the diagnostic
// would have been reported at, and the human message. Callers can
// subtract these from a prior Lint() result to obtain the diagnostics
// still standing after the fix.
type FixedDiagnostic struct {
	Code string
	Pos  ast.Pos
	Msg  string
}

// Fix applies every safe auto-fix to mod and returns the (mutated)
// module plus the list of diagnostics it resolved. mod's body slice
// and the affected Import / ImportFrom nodes are mutated in place;
// statements that don't change keep their pointer identity, which is
// what cst.Trivia attaches to.
//
// v0.1.14 ships only F401 fixes. F811 and F841 stay hands-off:
// removing the first binding (F811) could drop intentional decorator
// or registration side effects; removing an unused local (F841)
// could drop side effects of the right-hand side. Pyflakes / ruff
// also leave both alone.
//
// Fix recurses into module-level control flow (`if`, `try`, `with`,
// `for`, `while`, `match`) because those don't introduce scopes — an
// `import` inside `if posix:` binds at module scope and F401 flags
// it, so Fix needs to reach it too. `def` and `class` bodies *do*
// introduce scopes; their imports stay untouched.
func Fix(mod *ast.Module) (*ast.Module, []FixedDiagnostic) {
	if mod == nil {
		return mod, nil
	}
	sm := symbols.Build(mod)
	if sm.Root == nil {
		return mod, nil
	}
	f := &fixer{
		root:   sm.Root,
		exempt: futureImportNames(mod),
	}
	mod.Body = f.fixStmts(mod.Body)
	return mod, f.fixed
}

type fixer struct {
	root   *symbols.Scope
	exempt map[string]bool
	fixed  []FixedDiagnostic
}

func (f *fixer) usedAt(name string) (ast.Pos, bool) {
	sym, ok := f.root.Symbols[name]
	if !ok {
		return ast.Pos{}, false
	}
	if sym.Has(symbols.FlagUsed) {
		return ast.Pos{}, false
	}
	if len(sym.BindSites) == 0 {
		return ast.Pos{}, false
	}
	return sym.BindSites[0], true
}

// fixStmts filters and rewrites statements in scope-preserving
// containers (module body, if-branch, try-block, etc.). It does NOT
// descend into FunctionDef / ClassDef bodies because those introduce
// new symbol scopes that the module-scope F401 check ignores.
func (f *fixer) fixStmts(stmts []ast.StmtNode) []ast.StmtNode {
	out := stmts[:0]
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.Import:
			if kept := f.filterImport(n); len(kept) == 0 {
				continue
			} else {
				n.Names = kept
				out = append(out, n)
			}
		case *ast.ImportFrom:
			if n.Module == "__future__" {
				out = append(out, n)
				continue
			}
			if kept := f.filterImportFrom(n); len(kept) == 0 {
				continue
			} else {
				n.Names = kept
				out = append(out, n)
			}
		case *ast.If:
			n.Body = f.fixStmts(n.Body)
			n.Orelse = f.fixStmts(n.Orelse)
			out = append(out, n)
		case *ast.Try:
			n.Body = f.fixStmts(n.Body)
			for _, h := range n.Handlers {
				if eh, ok := h.(*ast.ExceptHandler); ok {
					eh.Body = f.fixStmts(eh.Body)
				}
			}
			n.Orelse = f.fixStmts(n.Orelse)
			n.Finalbody = f.fixStmts(n.Finalbody)
			out = append(out, n)
		case *ast.TryStar:
			n.Body = f.fixStmts(n.Body)
			for _, h := range n.Handlers {
				if eh, ok := h.(*ast.ExceptHandler); ok {
					eh.Body = f.fixStmts(eh.Body)
				}
			}
			n.Orelse = f.fixStmts(n.Orelse)
			n.Finalbody = f.fixStmts(n.Finalbody)
			out = append(out, n)
		case *ast.With:
			n.Body = f.fixStmts(n.Body)
			out = append(out, n)
		case *ast.AsyncWith:
			n.Body = f.fixStmts(n.Body)
			out = append(out, n)
		case *ast.For:
			n.Body = f.fixStmts(n.Body)
			n.Orelse = f.fixStmts(n.Orelse)
			out = append(out, n)
		case *ast.AsyncFor:
			n.Body = f.fixStmts(n.Body)
			n.Orelse = f.fixStmts(n.Orelse)
			out = append(out, n)
		case *ast.While:
			n.Body = f.fixStmts(n.Body)
			n.Orelse = f.fixStmts(n.Orelse)
			out = append(out, n)
		case *ast.Match:
			for _, mc := range n.Cases {
				mc.Body = f.fixStmts(mc.Body)
			}
			out = append(out, n)
		default:
			out = append(out, s)
		}
	}
	return out
}

func (f *fixer) filterImport(n *ast.Import) []*ast.Alias {
	kept := n.Names[:0]
	for _, a := range n.Names {
		name := a.Asname
		if name == "" {
			name = topImportName(a.Name)
		}
		if f.exempt[name] {
			kept = append(kept, a)
			continue
		}
		pos, unused := f.usedAt(name)
		if !unused {
			kept = append(kept, a)
			continue
		}
		f.fixed = append(f.fixed, FixedDiagnostic{
			Code: CodeUnusedImport,
			Pos:  pos,
			Msg:  "'" + name + "' imported but unused",
		})
	}
	return kept
}

func (f *fixer) filterImportFrom(n *ast.ImportFrom) []*ast.Alias {
	kept := n.Names[:0]
	for _, a := range n.Names {
		if a.Name == "*" {
			kept = append(kept, a)
			continue
		}
		name := a.Asname
		if name == "" {
			name = a.Name
		}
		pos, unused := f.usedAt(name)
		if !unused {
			kept = append(kept, a)
			continue
		}
		f.fixed = append(f.fixed, FixedDiagnostic{
			Code: CodeUnusedImport,
			Pos:  pos,
			Msg:  "'" + name + "' imported but unused",
		})
	}
	return kept
}

// topImportName picks the binding name for `import a.b.c` — the
// first dotted component. Mirrors symbols.topImportName so the
// linter doesn't have to import an internal helper.
func topImportName(dotted string) string {
	for i := 0; i < len(dotted); i++ {
		if dotted[i] == '.' {
			return dotted[:i]
		}
	}
	return dotted
}
