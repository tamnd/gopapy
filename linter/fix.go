package linter

import (
	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/symbols"
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
// v0.1.15 adds one F811 shape: a dead store of a literal in a
// function body where the immediately-next statement rebinds the
// same name. F841 still stays hands-off — removing an unused local
// could drop side effects of the right-hand side; pyflakes/ruff also
// leave it alone.
//
// Fix recurses into module-level control flow (`if`, `try`, `with`,
// `for`, `while`, `match`) for F401 because those don't introduce
// scopes — an `import` inside `if posix:` binds at module scope and
// F401 flags it, so Fix needs to reach it too. The F811 dead-store
// pass walks the entire tree to find every FunctionDef/AsyncFunctionDef
// body; it scans the function's top-level statement list only.
//
// Equivalent to FixWithConfig(mod, Config{}, "").
func Fix(mod *ast.Module) (*ast.Module, []FixedDiagnostic) {
	return FixWithConfig(mod, Config{}, "")
}

// FixWithConfig is Fix gated by a Config. A given fix runs only when
// its code is enabled for filename under cfg, so a project that
// ignores F401 for `tests/*` won't have its test imports rewritten
// under `gopapy lint --fix`. Pass an empty filename to skip per-file
// gating; the global Select / Ignore lists still apply.
func FixWithConfig(mod *ast.Module, cfg Config, filename string) (*ast.Module, []FixedDiagnostic) {
	if mod == nil {
		return mod, nil
	}
	sm := symbols.Build(mod)
	if sm.Root == nil {
		return mod, nil
	}
	f := &fixer{
		root:     sm.Root,
		exempt:   futureImportNames(mod),
		cfg:      cfg,
		filename: filename,
	}
	if f.codeEnabled(CodeUnusedImport) {
		mod.Body = f.fixStmts(mod.Body)
	}
	if f.codeEnabled(CodeRedefinitionUnused) {
		f.applyDeadStoreLiteralFix(mod)
	}
	noneOn := f.codeEnabled(CodeComparisonToNone)
	boolOn := f.codeEnabled(CodeComparisonToBool)
	if noneOn || boolOn {
		f.applyComparisonIdentityFix(mod, noneOn, boolOn)
	}
	return mod, f.fixed
}

type fixer struct {
	root     *symbols.Scope
	exempt   map[string]bool
	cfg      Config
	filename string
	fixed    []FixedDiagnostic
}

func (f *fixer) codeEnabled(code string) bool {
	if f.filename == "" {
		return f.cfg.Enabled(code)
	}
	return f.cfg.EnabledFor(f.filename, code)
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

// applyDeadStoreLiteralFix removes `name = CONSTANT` statements in
// function/method bodies when the very next statement rebinds `name`.
// Because the two statements are adjacent, no read of `name` can sit
// between them, so dropping the first preserves observable behavior.
// The constant-RHS guard rules out side effects.
//
// The pass restricts itself to the top level of each FunctionDef /
// AsyncFunctionDef body. F811 inside nested if/try/while can be fixed
// in a later version; the conservative shape covers the common case
// (a stale default a few lines above the real assignment) without
// risking branch-aware reasoning.
func (f *fixer) applyDeadStoreLiteralFix(mod *ast.Module) {
	ast.WalkPreorder(mod, func(n ast.Node) {
		switch fn := n.(type) {
		case *ast.FunctionDef:
			fn.Body = f.scanDeadStoreLiteral(fn.Body)
		case *ast.AsyncFunctionDef:
			fn.Body = f.scanDeadStoreLiteral(fn.Body)
		}
	})
}

func (f *fixer) scanDeadStoreLiteral(stmts []ast.StmtNode) []ast.StmtNode {
	out := make([]ast.StmtNode, 0, len(stmts))
	for i := 0; i < len(stmts); i++ {
		s := stmts[i]
		if i+1 < len(stmts) {
			if name, ok := constantStoreName(s); ok {
				if pos, ok := nextRebindPos(stmts[i+1], name); ok {
					f.fixed = append(f.fixed, FixedDiagnostic{
						Code: CodeRedefinitionUnused,
						Pos:  pos,
						Msg:  "redefinition of unused '" + name + "'",
					})
					continue
				}
			}
		}
		out = append(out, s)
	}
	return out
}

// constantStoreName matches `name = CONSTANT`: a single-target Assign
// whose target is a bare Name in Store context and whose value is an
// *ast.Constant (no side effects).
func constantStoreName(s ast.StmtNode) (string, bool) {
	a, ok := s.(*ast.Assign)
	if !ok || len(a.Targets) != 1 {
		return "", false
	}
	nm, ok := a.Targets[0].(*ast.Name)
	if !ok {
		return "", false
	}
	if _, isStore := nm.Ctx.(*ast.Store); !isStore {
		return "", false
	}
	if _, ok := a.Value.(*ast.Constant); !ok {
		return "", false
	}
	return nm.Id, true
}

// nextRebindPos returns the position F811 would report at if `s`
// rebinds `want` in the same scope. Accepts Assign and AnnAssign
// (with a value); AugAssign is excluded because it reads the target
// before writing, which would have already disqualified F811.
func nextRebindPos(s ast.StmtNode, want string) (ast.Pos, bool) {
	switch n := s.(type) {
	case *ast.Assign:
		if len(n.Targets) != 1 {
			return ast.Pos{}, false
		}
		nm, ok := n.Targets[0].(*ast.Name)
		if !ok || nm.Id != want {
			return ast.Pos{}, false
		}
		if _, isStore := nm.Ctx.(*ast.Store); !isStore {
			return ast.Pos{}, false
		}
		return nm.Pos, true
	case *ast.AnnAssign:
		if n.Value == nil {
			return ast.Pos{}, false
		}
		nm, ok := n.Target.(*ast.Name)
		if !ok || nm.Id != want {
			return ast.Pos{}, false
		}
		return nm.Pos, true
	}
	return ast.Pos{}, false
}

// applyComparisonIdentityFix rewrites `==`/`!=` to `is`/`is not`
// for E711 (comparison to None) and E712 (comparison to True/False).
// The transform mutates the Compare node's Ops slice in place; only
// the operator slot facing a None/Bool literal is rewritten, so a
// chained comparison like `a == None == b` rewrites both slots
// independently without affecting an unrelated middle.
//
// The bool case skips the truthiness rewrite (`x == True` → `x`):
// that's behavior-changing for non-bool truthy values and pycodestyle
// leaves it as a separate concern.
func (f *fixer) applyComparisonIdentityFix(mod *ast.Module, noneOn, boolOn bool) {
	ast.WalkPreorder(mod, func(n ast.Node) {
		c, ok := n.(*ast.Compare)
		if !ok {
			return
		}
		left := c.Left
		for i, op := range c.Ops {
			if i >= len(c.Comparators) {
				break
			}
			right := c.Comparators[i]
			if !isEqOrNotEq(op) {
				left = right
				continue
			}
			noneSide := isNoneConstant(left) || isNoneConstant(right)
			boolSide := isBoolConstant(left) || isBoolConstant(right)
			switch {
			case noneOn && noneSide:
				c.Ops[i] = rewriteEqToIs(op)
				f.fixed = append(f.fixed, FixedDiagnostic{
					Code: CodeComparisonToNone,
					Pos:  c.Pos,
					Msg:  "comparison to None should be `if cond is None:`",
				})
			case boolOn && boolSide:
				c.Ops[i] = rewriteEqToIs(op)
				f.fixed = append(f.fixed, FixedDiagnostic{
					Code: CodeComparisonToBool,
					Pos:  c.Pos,
					Msg:  "comparison to True/False should be `if cond is True:` or `if cond:`",
				})
			}
			left = right
		}
	})
}

// rewriteEqToIs maps Eq → Is and NotEq → IsNot. Cmpop nodes are
// position-less in this AST, so a fresh node is enough — the unparser
// derives spelling from the type. Other op types are returned
// unchanged; callers gate this with isEqOrNotEq.
func rewriteEqToIs(op ast.CmpopNode) ast.CmpopNode {
	switch op.(type) {
	case *ast.Eq:
		return &ast.Is{}
	case *ast.NotEq:
		return &ast.IsNot{}
	}
	return op
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
