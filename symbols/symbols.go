// Package symbols computes a Python symbol table from an AST module.
//
// The output mirrors what CPython's `_symtable` module produces: a tree
// of scopes (module, function, class, lambda, comprehension) with each
// name in a scope classified as local, global, nonlocal, free, cell, or
// parameter.
//
// Build never panics on a well-formed AST. Semantic problems with the
// source (a name declared both global and nonlocal, a nonlocal binding
// with no enclosing function definition, etc.) are reported as
// diagnostics on the returned Module rather than as errors.
package symbols

import (
	"github.com/tamnd/gopapy/v1/ast"
)

// Module is the top-level result of Build. The root scope mirrors the
// Python module being analysed.
type Module struct {
	Root        *Scope
	Diagnostics []Diagnostic
}

// ScopeKind discriminates between the kinds of scope Python supports.
type ScopeKind int

const (
	ScopeModule ScopeKind = iota
	ScopeFunction
	ScopeClass
	ScopeLambda
	ScopeComprehension
)

// String renders ScopeKind for diagnostics.
func (k ScopeKind) String() string {
	switch k {
	case ScopeModule:
		return "module"
	case ScopeFunction:
		return "function"
	case ScopeClass:
		return "class"
	case ScopeLambda:
		return "lambda"
	case ScopeComprehension:
		return "comprehension"
	}
	return "?"
}

// Scope is one lexical scope.
type Scope struct {
	Kind     ScopeKind
	Name     string // identifier of the def/class/lambda; empty for module
	Pos      ast.Pos
	Parent   *Scope
	Children []*Scope
	// Symbols indexed by identifier. A name that's both bound and used
	// has a single Binding entry covering both — see Binding.Flags for
	// the classification.
	Symbols map[string]*Binding
}

// BindFlag is a bitfield describing how a name is used in a scope.
type BindFlag uint16

const (
	FlagBound      BindFlag = 1 << iota // assigned to in this scope
	FlagUsed                            // referenced (load context) in this scope
	FlagParam                           // function parameter
	FlagGlobal                          // explicit `global` declaration
	FlagNonlocal                        // explicit `nonlocal` declaration
	FlagAnnotation                      // referenced only in an annotation
	FlagImport                          // bound by an import statement
	FlagFree                            // resolved from an enclosing function scope
	FlagCell                            // bound here and captured by an inner function
)

// Has reports whether the binding carries flag.
func (b *Binding) Has(flag BindFlag) bool { return b.Flags&flag != 0 }

// Binding is one name in one scope, together with where it was bound
// and how it's classified.
type Binding struct {
	Name      string
	Flags     BindFlag
	BindSites []ast.Pos // every assignment / parameter / for-target / etc.
	UseSites  []ast.Pos // load-context references
}

// Diagnostic is a non-fatal semantic problem detected during scope
// resolution.
type Diagnostic struct {
	Pos ast.Pos
	Msg string
}

// Build walks mod and returns the symbol table.
func Build(mod *ast.Module) *Module {
	root := newScope(ScopeModule, "", ast.Pos{})
	b := &builder{cur: root, root: root}
	for _, s := range mod.Body {
		b.stmt(s)
	}
	b.resolve(root)
	return &Module{Root: root, Diagnostics: b.diagnostics}
}

func newScope(kind ScopeKind, name string, pos ast.Pos) *Scope {
	return &Scope{Kind: kind, Name: name, Pos: pos, Symbols: map[string]*Binding{}}
}

type builder struct {
	root        *Scope
	cur         *Scope
	diagnostics []Diagnostic
}

func (b *builder) push(kind ScopeKind, name string, pos ast.Pos) *Scope {
	child := newScope(kind, name, pos)
	child.Parent = b.cur
	b.cur.Children = append(b.cur.Children, child)
	b.cur = child
	return child
}

func (b *builder) pop() {
	if b.cur.Parent != nil {
		b.cur = b.cur.Parent
	}
}

func (b *builder) bind(scope *Scope, name string, pos ast.Pos, flag BindFlag) *Binding {
	if name == "" {
		return nil
	}
	sym := scope.Symbols[name]
	if sym == nil {
		sym = &Binding{Name: name}
		scope.Symbols[name] = sym
	}
	sym.Flags |= flag | FlagBound
	sym.BindSites = append(sym.BindSites, pos)
	return sym
}

func (b *builder) use(scope *Scope, name string, pos ast.Pos) *Binding {
	if name == "" {
		return nil
	}
	sym := scope.Symbols[name]
	if sym == nil {
		sym = &Binding{Name: name}
		scope.Symbols[name] = sym
	}
	sym.Flags |= FlagUsed
	sym.UseSites = append(sym.UseSites, pos)
	return sym
}

func (b *builder) declare(scope *Scope, name string, pos ast.Pos, flag BindFlag) {
	sym := scope.Symbols[name]
	if sym == nil {
		sym = &Binding{Name: name}
		scope.Symbols[name] = sym
	}
	if flag == FlagGlobal && sym.Has(FlagNonlocal) {
		b.diagnostics = append(b.diagnostics, Diagnostic{Pos: pos, Msg: "name " + name + " is both nonlocal and global"})
	}
	if flag == FlagNonlocal && sym.Has(FlagGlobal) {
		b.diagnostics = append(b.diagnostics, Diagnostic{Pos: pos, Msg: "name " + name + " is both global and nonlocal"})
	}
	sym.Flags |= flag
}

// stmt walks a statement, dispatching on type. Walking expressions
// goes through expr; nested scopes (def, class, lambda, comprehensions)
// push a new Scope on b.cur for the duration of the body.
func (b *builder) stmt(s ast.StmtNode) {
	if s == nil {
		return
	}
	switch n := s.(type) {
	case *ast.FunctionDef:
		b.funcDef(n.Name, n.Pos, n.Args, n.Body, n.DecoratorList, n.Returns, n.TypeParams, ScopeFunction)
	case *ast.AsyncFunctionDef:
		b.funcDef(n.Name, n.Pos, n.Args, n.Body, n.DecoratorList, n.Returns, n.TypeParams, ScopeFunction)
	case *ast.ClassDef:
		b.classDef(n)
	case *ast.Return:
		b.expr(n.Value)
	case *ast.Delete:
		for _, t := range n.Targets {
			b.target(t, FlagBound)
		}
	case *ast.Assign:
		b.expr(n.Value)
		for _, t := range n.Targets {
			b.target(t, FlagBound)
		}
	case *ast.TypeAlias:
		b.target(n.Name, FlagBound)
		b.expr(n.Value)
	case *ast.AugAssign:
		b.target(n.Target, FlagBound)
		b.expr(n.Value)
	case *ast.AnnAssign:
		b.target(n.Target, FlagBound|FlagAnnotation)
		b.expr(n.Annotation)
		b.expr(n.Value)
	case *ast.For:
		b.target(n.Target, FlagBound)
		b.expr(n.Iter)
		b.stmts(n.Body)
		b.stmts(n.Orelse)
	case *ast.AsyncFor:
		b.target(n.Target, FlagBound)
		b.expr(n.Iter)
		b.stmts(n.Body)
		b.stmts(n.Orelse)
	case *ast.While:
		b.expr(n.Test)
		b.stmts(n.Body)
		b.stmts(n.Orelse)
	case *ast.If:
		b.expr(n.Test)
		b.stmts(n.Body)
		b.stmts(n.Orelse)
	case *ast.With:
		for _, item := range n.Items {
			b.expr(item.ContextExpr)
			if item.OptionalVars != nil {
				b.target(item.OptionalVars, FlagBound)
			}
		}
		b.stmts(n.Body)
	case *ast.AsyncWith:
		for _, item := range n.Items {
			b.expr(item.ContextExpr)
			if item.OptionalVars != nil {
				b.target(item.OptionalVars, FlagBound)
			}
		}
		b.stmts(n.Body)
	case *ast.Match:
		b.expr(n.Subject)
		for _, c := range n.Cases {
			b.pattern(c.Pattern)
			b.expr(c.Guard)
			b.stmts(c.Body)
		}
	case *ast.Raise:
		b.expr(n.Exc)
		b.expr(n.Cause)
	case *ast.Try:
		b.stmts(n.Body)
		for _, h := range n.Handlers {
			b.except(h)
		}
		b.stmts(n.Orelse)
		b.stmts(n.Finalbody)
	case *ast.TryStar:
		b.stmts(n.Body)
		for _, h := range n.Handlers {
			b.except(h)
		}
		b.stmts(n.Orelse)
		b.stmts(n.Finalbody)
	case *ast.Assert:
		b.expr(n.Test)
		b.expr(n.Msg)
	case *ast.Import:
		for _, a := range n.Names {
			name := a.Asname
			if name == "" {
				name = topImportName(a.Name)
			}
			b.bind(b.cur, name, n.Pos, FlagImport)
		}
	case *ast.ImportFrom:
		for _, a := range n.Names {
			if a.Name == "*" {
				continue
			}
			name := a.Asname
			if name == "" {
				name = a.Name
			}
			b.bind(b.cur, name, n.Pos, FlagImport)
		}
	case *ast.Global:
		for _, name := range n.Names {
			b.declare(b.cur, name, n.Pos, FlagGlobal)
		}
	case *ast.Nonlocal:
		for _, name := range n.Names {
			b.declare(b.cur, name, n.Pos, FlagNonlocal)
		}
	case *ast.Expr:
		b.expr(n.Value)
	}
}

func (b *builder) stmts(ss []ast.StmtNode) {
	for _, s := range ss {
		b.stmt(s)
	}
}

// funcDef walks a function definition: decorators and defaults are
// evaluated in the *enclosing* scope; parameters and body live in the
// new function scope. Type params introduce their own implicit scope
// in CPython 3.12+, but for v0.1.4 simplicity they're bound in the
// function scope itself.
func (b *builder) funcDef(name string, pos ast.Pos, args *ast.Arguments, body []ast.StmtNode, decorators []ast.ExprNode, returns ast.ExprNode, typeParams []ast.TypeParamNode, kind ScopeKind) {
	b.bind(b.cur, name, pos, FlagBound)
	for _, d := range decorators {
		b.expr(d)
	}
	if args != nil {
		for _, def := range args.Defaults {
			b.expr(def)
		}
		for _, def := range args.KwDefaults {
			b.expr(def)
		}
	}
	b.expr(returns)

	scope := b.push(kind, name, pos)
	for _, tp := range typeParams {
		b.bindTypeParam(scope, tp)
	}
	if args != nil {
		b.params(scope, args)
	}
	b.stmts(body)
	b.pop()
}

// params binds every formal parameter into scope and walks each
// annotation expression in scope (annotations are evaluated lazily in
// CPython but their names are visible to the function body).
func (b *builder) params(scope *Scope, args *ast.Arguments) {
	for _, a := range args.Posonlyargs {
		b.bind(scope, a.Arg, a.Pos, FlagParam)
		b.expr(a.Annotation)
	}
	for _, a := range args.Args {
		b.bind(scope, a.Arg, a.Pos, FlagParam)
		b.expr(a.Annotation)
	}
	if args.Vararg != nil {
		b.bind(scope, args.Vararg.Arg, args.Vararg.Pos, FlagParam)
		b.expr(args.Vararg.Annotation)
	}
	for _, a := range args.Kwonlyargs {
		b.bind(scope, a.Arg, a.Pos, FlagParam)
		b.expr(a.Annotation)
	}
	if args.Kwarg != nil {
		b.bind(scope, args.Kwarg.Arg, args.Kwarg.Pos, FlagParam)
		b.expr(args.Kwarg.Annotation)
	}
}

func (b *builder) bindTypeParam(scope *Scope, tp ast.TypeParamNode) {
	switch n := tp.(type) {
	case *ast.TypeVar:
		b.bind(scope, n.Name, n.Pos, FlagBound)
		b.expr(n.Bound)
		b.expr(n.DefaultValue)
	case *ast.ParamSpec:
		b.bind(scope, n.Name, n.Pos, FlagBound)
		b.expr(n.DefaultValue)
	case *ast.TypeVarTuple:
		b.bind(scope, n.Name, n.Pos, FlagBound)
		b.expr(n.DefaultValue)
	}
}

// classDef binds the class name in the enclosing scope, then walks
// bases, keywords, and decorators in the enclosing scope. The class
// body lives in a class scope whose name lookups are special — Python
// resolves free variables by skipping the class scope.
func (b *builder) classDef(n *ast.ClassDef) {
	b.bind(b.cur, n.Name, n.Pos, FlagBound)
	for _, d := range n.DecoratorList {
		b.expr(d)
	}
	for _, base := range n.Bases {
		b.expr(base)
	}
	for _, kw := range n.Keywords {
		b.expr(kw.Value)
	}
	scope := b.push(ScopeClass, n.Name, n.Pos)
	for _, tp := range n.TypeParams {
		b.bindTypeParam(scope, tp)
	}
	b.stmts(n.Body)
	b.pop()
}

// except handles `except E as e:` — the binding is scoped to the body
// of the handler in CPython but for the symbol table we just record it
// as a normal binding in the enclosing scope.
func (b *builder) except(h ast.ExcepthandlerNode) {
	eh, ok := h.(*ast.ExceptHandler)
	if !ok {
		return
	}
	b.expr(eh.Type)
	if eh.Name != "" {
		b.bind(b.cur, eh.Name, eh.Pos, FlagBound)
	}
	b.stmts(eh.Body)
}

// expr walks an expression and records load-context name uses. Nested
// scope-introducing expressions (Lambda, comprehensions) push a new
// scope for the duration of their body.
func (b *builder) expr(e ast.ExprNode) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.BoolOp:
		for _, v := range n.Values {
			b.expr(v)
		}
	case *ast.NamedExpr:
		b.expr(n.Value)
		// PEP 572: walrus targets bind in the enclosing function/module
		// scope when used inside a comprehension. We handle that by
		// finding the nearest non-comprehension ancestor.
		target, _ := n.Target.(*ast.Name)
		if target == nil {
			return
		}
		scope := b.cur
		for scope.Parent != nil && scope.Kind == ScopeComprehension {
			scope = scope.Parent
		}
		b.bind(scope, target.Id, target.Pos, FlagBound)
	case *ast.BinOp:
		b.expr(n.Left)
		b.expr(n.Right)
	case *ast.UnaryOp:
		b.expr(n.Operand)
	case *ast.Lambda:
		scope := b.push(ScopeLambda, "<lambda>", n.Pos)
		if n.Args != nil {
			b.params(scope, n.Args)
		}
		b.expr(n.Body)
		b.pop()
	case *ast.IfExp:
		b.expr(n.Test)
		b.expr(n.Body)
		b.expr(n.Orelse)
	case *ast.Dict:
		for _, k := range n.Keys {
			b.expr(k)
		}
		for _, v := range n.Values {
			b.expr(v)
		}
	case *ast.Set:
		for _, v := range n.Elts {
			b.expr(v)
		}
	case *ast.ListComp:
		b.comp(n.Pos, n.Generators, func() { b.expr(n.Elt) })
	case *ast.SetComp:
		b.comp(n.Pos, n.Generators, func() { b.expr(n.Elt) })
	case *ast.DictComp:
		b.comp(n.Pos, n.Generators, func() { b.expr(n.Key); b.expr(n.Value) })
	case *ast.GeneratorExp:
		b.comp(n.Pos, n.Generators, func() { b.expr(n.Elt) })
	case *ast.Await:
		b.expr(n.Value)
	case *ast.Yield:
		b.expr(n.Value)
	case *ast.YieldFrom:
		b.expr(n.Value)
	case *ast.Compare:
		b.expr(n.Left)
		for _, c := range n.Comparators {
			b.expr(c)
		}
	case *ast.Call:
		b.expr(n.Func)
		for _, a := range n.Args {
			b.expr(a)
		}
		for _, kw := range n.Keywords {
			b.expr(kw.Value)
		}
	case *ast.FormattedValue:
		b.expr(n.Value)
		b.expr(n.FormatSpec)
	case *ast.Interpolation:
		b.expr(n.Value)
		b.expr(n.FormatSpec)
	case *ast.JoinedStr:
		for _, v := range n.Values {
			b.expr(v)
		}
	case *ast.TemplateStr:
		for _, v := range n.Values {
			b.expr(v)
		}
	case *ast.Attribute:
		b.expr(n.Value)
	case *ast.Subscript:
		b.expr(n.Value)
		b.expr(n.Slice)
	case *ast.Starred:
		b.expr(n.Value)
	case *ast.Name:
		switch n.Ctx.(type) {
		case *ast.Load:
			b.use(b.cur, n.Id, n.Pos)
		case *ast.Store:
			b.bind(b.cur, n.Id, n.Pos, FlagBound)
		case *ast.Del:
			b.bind(b.cur, n.Id, n.Pos, FlagBound)
			b.use(b.cur, n.Id, n.Pos)
		}
	case *ast.List:
		for _, v := range n.Elts {
			b.expr(v)
		}
	case *ast.Tuple:
		for _, v := range n.Elts {
			b.expr(v)
		}
	case *ast.Slice:
		b.expr(n.Lower)
		b.expr(n.Upper)
		b.expr(n.Step)
	}
}

// comp pushes a comprehension scope for the duration of the
// generators and elt walk. The first iterable is evaluated in the
// *enclosing* scope per CPython semantics — we approximate by walking
// it before the push.
func (b *builder) comp(pos ast.Pos, gens []*ast.Comprehension, elt func()) {
	if len(gens) == 0 {
		elt()
		return
	}
	b.expr(gens[0].Iter)
	scope := b.push(ScopeComprehension, "<comp>", pos)
	b.target(gens[0].Target, FlagBound)
	for _, ifx := range gens[0].Ifs {
		b.expr(ifx)
	}
	for _, g := range gens[1:] {
		b.expr(g.Iter)
		b.target(g.Target, FlagBound)
		for _, ifx := range g.Ifs {
			b.expr(ifx)
		}
	}
	elt()
	b.pop()
	_ = scope
}

// target walks an assignment target, recording every Name node found
// as a binding. Tuple/List/Starred wrappers recurse; Subscript and
// Attribute targets evaluate the receiver as a use, not a bind.
func (b *builder) target(e ast.ExprNode, flag BindFlag) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.Name:
		b.bind(b.cur, n.Id, n.Pos, flag)
	case *ast.Tuple:
		for _, x := range n.Elts {
			b.target(x, flag)
		}
	case *ast.List:
		for _, x := range n.Elts {
			b.target(x, flag)
		}
	case *ast.Starred:
		b.target(n.Value, flag)
	case *ast.Attribute:
		b.expr(n.Value)
	case *ast.Subscript:
		b.expr(n.Value)
		b.expr(n.Slice)
	default:
		// Anything else in target position is an unusual shape (e.g.
		// participle leaving a bare expression). Walk it as an expr so
		// embedded uses still get recorded.
		b.expr(e)
	}
}

// pattern walks a match pattern, binding every captured name. Class
// and value patterns evaluate their expressions in the enclosing scope.
func (b *builder) pattern(p ast.PatternNode) {
	if p == nil {
		return
	}
	switch n := p.(type) {
	case *ast.MatchValue:
		b.expr(n.Value)
	case *ast.MatchSingleton:
	case *ast.MatchSequence:
		for _, q := range n.Patterns {
			b.pattern(q)
		}
	case *ast.MatchMapping:
		for _, k := range n.Keys {
			b.expr(k)
		}
		for _, q := range n.Patterns {
			b.pattern(q)
		}
		if n.Rest != "" {
			b.bind(b.cur, n.Rest, n.Pos, FlagBound)
		}
	case *ast.MatchClass:
		b.expr(n.Cls)
		for _, q := range n.Patterns {
			b.pattern(q)
		}
		for _, q := range n.KwdPatterns {
			b.pattern(q)
		}
	case *ast.MatchStar:
		if n.Name != "" {
			b.bind(b.cur, n.Name, n.Pos, FlagBound)
		}
	case *ast.MatchAs:
		b.pattern(n.Pattern)
		if n.Name != "" {
			b.bind(b.cur, n.Name, n.Pos, FlagBound)
		}
	case *ast.MatchOr:
		for _, q := range n.Patterns {
			b.pattern(q)
		}
	}
}

// resolve walks the scope tree and computes Free / Cell flags. A name
// used in a function or comprehension scope but not bound there is
// looked up in enclosing function/module scopes; if found in a
// function scope, both sites are flagged (Free in the inner, Cell in
// the outer). Class scopes are skipped per Python semantics.
func (b *builder) resolve(scope *Scope) {
	for _, child := range scope.Children {
		b.resolve(child)
	}
	if scope.Kind == ScopeModule || scope.Kind == ScopeClass {
		// Module-level free names are just unbound at module load; class
		// bodies don't capture from the enclosing function scope.
		return
	}
	for name, sym := range scope.Symbols {
		if !sym.Has(FlagUsed) {
			continue
		}
		if sym.Has(FlagBound) || sym.Has(FlagGlobal) || sym.Has(FlagNonlocal) {
			continue
		}
		if owner := lookupCapturable(scope.Parent, name); owner != nil {
			sym.Flags |= FlagFree
			owner.Symbols[name].Flags |= FlagCell
		}
	}
}

// lookupCapturable walks up from start looking for a function scope
// that binds name. Class scopes are skipped (Python class bodies are
// not captured by inner functions). Module scope binds count as a
// free-var resolution but not as a Cell — the binding is global.
func lookupCapturable(start *Scope, name string) *Scope {
	for s := start; s != nil; s = s.Parent {
		if s.Kind == ScopeClass {
			continue
		}
		sym, ok := s.Symbols[name]
		if !ok {
			continue
		}
		if !sym.Has(FlagBound) {
			continue
		}
		if s.Kind == ScopeModule {
			return nil
		}
		return s
	}
	return nil
}

// topImportName picks the binding name for `import a.b.c` — it's the
// first dotted component (`a`).
func topImportName(dotted string) string {
	for i := 0; i < len(dotted); i++ {
		if dotted[i] == '.' {
			return dotted[:i]
		}
	}
	return dotted
}

// Resolve looks up name starting from scope, walking up the parent
// chain to find the scope that actually *binds* the name (a Used-only
// symbol entry counts as a free reference, not a binding). The boolean
// is true when the resolution crossed a function boundary (so the
// caller knows it's a free-variable / closure use). Class scopes other
// than the starting one are skipped per Python's nested-scope rule.
func (s *Scope) Resolve(name string) (*Binding, bool, *Scope) {
	crossed := false
	for cur := s; cur != nil; cur = cur.Parent {
		if cur != s && cur.Kind == ScopeClass {
			continue
		}
		if sym, ok := cur.Symbols[name]; ok {
			if sym.Has(FlagBound) || sym.Has(FlagGlobal) || sym.Has(FlagNonlocal) {
				return sym, crossed, cur
			}
		}
		if cur.Kind == ScopeFunction || cur.Kind == ScopeLambda || cur.Kind == ScopeComprehension {
			crossed = true
		}
	}
	return nil, false, nil
}
