// Package symbols2 computes a Python symbol table from a parser2 module.
//
// The output mirrors what CPython's `_symtable` module produces: a tree
// of scopes (module, function, class, lambda, comprehension) with each
// name in a scope classified as local, global, nonlocal, free, cell, or
// parameter.
//
// Build never panics on a well-formed AST. Semantic problems with the
// source are reported as diagnostics on the returned Module rather than
// as errors.
package symbols2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

// Module is the top-level result of Build.
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
	Name     string
	Pos      parser2.Pos
	Parent   *Scope
	Children []*Scope
	Symbols  map[string]*Binding
}

// BindFlag is a bitfield describing how a name is used in a scope.
type BindFlag uint16

const (
	FlagBound      BindFlag = 1 << iota
	FlagUsed
	FlagParam
	FlagGlobal
	FlagNonlocal
	FlagAnnotation
	FlagImport
	FlagFree
	FlagCell
)

// Has reports whether the binding carries flag.
func (b *Binding) Has(flag BindFlag) bool { return b.Flags&flag != 0 }

// Binding is one name in one scope.
type Binding struct {
	Name      string
	Flags     BindFlag
	BindSites []parser2.Pos
	UseSites  []parser2.Pos
}

// Diagnostic is a non-fatal semantic problem.
type Diagnostic = diag.Diagnostic

const (
	CodeGlobalAndNonlocal = "S001"
	CodeNonlocalNoBinding = "S002"
	CodeUsedBeforeAssign  = "S003"
)

// Build walks mod and returns the symbol table.
func Build(mod *parser2.Module) *Module {
	root := newScope(ScopeModule, "", parser2.Pos{})
	b := &builder{cur: root, root: root}
	for _, s := range mod.Body {
		b.stmt(s)
	}
	b.resolve(root)
	return &Module{Root: root, Diagnostics: b.diagnostics}
}

func newScope(kind ScopeKind, name string, pos parser2.Pos) *Scope {
	return &Scope{Kind: kind, Name: name, Pos: pos, Symbols: map[string]*Binding{}}
}

type builder struct {
	root        *Scope
	cur         *Scope
	diagnostics []Diagnostic
}

func (b *builder) push(kind ScopeKind, name string, pos parser2.Pos) *Scope {
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

func (b *builder) bind(scope *Scope, name string, pos parser2.Pos, flag BindFlag) *Binding {
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

func (b *builder) use(scope *Scope, name string, pos parser2.Pos) *Binding {
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

func (b *builder) declare(scope *Scope, name string, pos parser2.Pos, flag BindFlag) {
	sym := scope.Symbols[name]
	if sym == nil {
		sym = &Binding{Name: name}
		scope.Symbols[name] = sym
	}
	if flag == FlagGlobal && sym.Has(FlagNonlocal) {
		b.diagnostics = append(b.diagnostics, Diagnostic{
			Pos:      pos,
			End:      pos,
			Severity: diag.SeverityWarning,
			Code:     CodeGlobalAndNonlocal,
			Msg:      "name " + name + " is both nonlocal and global",
		})
	}
	if flag == FlagNonlocal && sym.Has(FlagGlobal) {
		b.diagnostics = append(b.diagnostics, Diagnostic{
			Pos:      pos,
			End:      pos,
			Severity: diag.SeverityWarning,
			Code:     CodeGlobalAndNonlocal,
			Msg:      "name " + name + " is both global and nonlocal",
		})
	}
	sym.Flags |= flag
}

func (b *builder) stmt(s parser2.Stmt) {
	if s == nil {
		return
	}
	switch n := s.(type) {
	case *parser2.FunctionDef:
		b.funcDef(n.Name, n.P, n.Args, n.Body, n.DecoratorList, n.Returns, n.TypeParams, ScopeFunction)
	case *parser2.AsyncFunctionDef:
		b.funcDef(n.Name, n.P, n.Args, n.Body, n.DecoratorList, n.Returns, n.TypeParams, ScopeFunction)
	case *parser2.ClassDef:
		b.classDef(n)
	case *parser2.Return:
		b.expr(n.Value)
	case *parser2.Delete:
		for _, t := range n.Targets {
			b.target(t, FlagBound)
		}
	case *parser2.Assign:
		b.expr(n.Value)
		for _, t := range n.Targets {
			b.target(t, FlagBound)
		}
	case *parser2.TypeAlias:
		b.target(n.Name, FlagBound)
		b.expr(n.Value)
	case *parser2.AugAssign:
		b.target(n.Target, FlagBound)
		b.augUse(n.Target)
		b.expr(n.Value)
	case *parser2.AnnAssign:
		b.target(n.Target, FlagBound|FlagAnnotation)
		b.expr(n.Annotation)
		b.expr(n.Value)
	case *parser2.For:
		b.target(n.Target, FlagBound)
		b.expr(n.Iter)
		b.stmts(n.Body)
		b.stmts(n.Orelse)
	case *parser2.AsyncFor:
		b.target(n.Target, FlagBound)
		b.expr(n.Iter)
		b.stmts(n.Body)
		b.stmts(n.Orelse)
	case *parser2.While:
		b.expr(n.Test)
		b.stmts(n.Body)
		b.stmts(n.Orelse)
	case *parser2.If:
		b.expr(n.Test)
		b.stmts(n.Body)
		b.stmts(n.Orelse)
	case *parser2.With:
		for _, item := range n.Items {
			b.expr(item.ContextExpr)
			if item.OptionalVars != nil {
				b.target(item.OptionalVars, FlagBound)
			}
		}
		b.stmts(n.Body)
	case *parser2.AsyncWith:
		for _, item := range n.Items {
			b.expr(item.ContextExpr)
			if item.OptionalVars != nil {
				b.target(item.OptionalVars, FlagBound)
			}
		}
		b.stmts(n.Body)
	case *parser2.Match:
		b.expr(n.Subject)
		for _, c := range n.Cases {
			b.pattern(c.Pattern)
			b.expr(c.Guard)
			b.stmts(c.Body)
		}
	case *parser2.Raise:
		b.expr(n.Exc)
		b.expr(n.Cause)
	case *parser2.Try:
		b.stmts(n.Body)
		for _, h := range n.Handlers {
			b.exceptHandler(h)
		}
		b.stmts(n.Orelse)
		b.stmts(n.Finalbody)
	case *parser2.Assert:
		b.expr(n.Test)
		b.expr(n.Msg)
	case *parser2.Import:
		for _, a := range n.Names {
			name := a.Asname
			if name == "" {
				name = topImportName(a.Name)
			}
			b.bind(b.cur, name, n.P, FlagImport)
		}
	case *parser2.ImportFrom:
		for _, a := range n.Names {
			if a.Name == "*" {
				continue
			}
			name := a.Asname
			if name == "" {
				name = a.Name
			}
			b.bind(b.cur, name, n.P, FlagImport)
		}
	case *parser2.Global:
		for _, name := range n.Names {
			b.declare(b.cur, name, n.P, FlagGlobal)
		}
	case *parser2.Nonlocal:
		for _, name := range n.Names {
			b.declare(b.cur, name, n.P, FlagNonlocal)
		}
	case *parser2.ExprStmt:
		b.expr(n.Value)
	}
}

func (b *builder) stmts(ss []parser2.Stmt) {
	for _, s := range ss {
		b.stmt(s)
	}
}

func (b *builder) funcDef(name string, pos parser2.Pos, args *parser2.Arguments, body []parser2.Stmt, decorators []parser2.Expr, returns parser2.Expr, typeParams []parser2.TypeParam, kind ScopeKind) {
	b.bind(b.cur, name, pos, FlagBound)
	for _, d := range decorators {
		b.expr(d)
	}
	if args != nil {
		for _, def := range args.Defaults {
			b.expr(def)
		}
		for _, def := range args.KwOnlyDef {
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

func (b *builder) params(scope *Scope, args *parser2.Arguments) {
	for _, a := range args.PosOnly {
		b.bind(scope, a.Name, a.P, FlagParam)
		b.expr(a.Annotation)
	}
	for _, a := range args.Args {
		b.bind(scope, a.Name, a.P, FlagParam)
		b.expr(a.Annotation)
	}
	if args.Vararg != nil {
		b.bind(scope, args.Vararg.Name, args.Vararg.P, FlagParam)
		b.expr(args.Vararg.Annotation)
	}
	for _, a := range args.KwOnly {
		b.bind(scope, a.Name, a.P, FlagParam)
		b.expr(a.Annotation)
	}
	if args.Kwarg != nil {
		b.bind(scope, args.Kwarg.Name, args.Kwarg.P, FlagParam)
		b.expr(args.Kwarg.Annotation)
	}
}

func (b *builder) bindTypeParam(scope *Scope, tp parser2.TypeParam) {
	switch n := tp.(type) {
	case *parser2.TypeVar:
		b.bind(scope, n.Name, n.P, FlagBound)
		b.expr(n.Bound)
		b.expr(n.DefaultValue)
	case *parser2.ParamSpec:
		b.bind(scope, n.Name, n.P, FlagBound)
		b.expr(n.DefaultValue)
	case *parser2.TypeVarTuple:
		b.bind(scope, n.Name, n.P, FlagBound)
		b.expr(n.DefaultValue)
	}
}

func (b *builder) classDef(n *parser2.ClassDef) {
	b.bind(b.cur, n.Name, n.P, FlagBound)
	for _, d := range n.DecoratorList {
		b.expr(d)
	}
	for _, base := range n.Bases {
		b.expr(base)
	}
	for _, kw := range n.Keywords {
		b.expr(kw.Value)
	}
	scope := b.push(ScopeClass, n.Name, n.P)
	for _, tp := range n.TypeParams {
		b.bindTypeParam(scope, tp)
	}
	b.stmts(n.Body)
	b.pop()
}

func (b *builder) exceptHandler(h *parser2.ExceptHandler) {
	if h == nil {
		return
	}
	b.expr(h.Type)
	if h.Name != "" {
		b.bind(b.cur, h.Name, h.P, FlagBound)
	}
	b.stmts(h.Body)
}

func (b *builder) expr(e parser2.Expr) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *parser2.BoolOp:
		for _, v := range n.Values {
			b.expr(v)
		}
	case *parser2.NamedExpr:
		b.expr(n.Value)
		target, _ := n.Target.(*parser2.Name)
		if target == nil {
			return
		}
		scope := b.cur
		for scope.Parent != nil && scope.Kind == ScopeComprehension {
			scope = scope.Parent
		}
		b.bind(scope, target.Id, target.P, FlagBound)
	case *parser2.BinOp:
		b.expr(n.Left)
		b.expr(n.Right)
	case *parser2.UnaryOp:
		b.expr(n.Operand)
	case *parser2.Lambda:
		scope := b.push(ScopeLambda, "<lambda>", n.P)
		if n.Args != nil {
			b.params(scope, n.Args)
		}
		b.expr(n.Body)
		b.pop()
	case *parser2.IfExp:
		b.expr(n.Test)
		b.expr(n.Body)
		b.expr(n.OrElse)
	case *parser2.Dict:
		for _, k := range n.Keys {
			b.expr(k)
		}
		for _, v := range n.Values {
			b.expr(v)
		}
	case *parser2.Set:
		for _, v := range n.Elts {
			b.expr(v)
		}
	case *parser2.ListComp:
		b.comp(n.P, n.Gens, func() { b.expr(n.Elt) })
	case *parser2.SetComp:
		b.comp(n.P, n.Gens, func() { b.expr(n.Elt) })
	case *parser2.DictComp:
		b.comp(n.P, n.Gens, func() { b.expr(n.Key); b.expr(n.Value) })
	case *parser2.GeneratorExp:
		b.comp(n.P, n.Gens, func() { b.expr(n.Elt) })
	case *parser2.Await:
		b.expr(n.Value)
	case *parser2.Yield:
		b.expr(n.Value)
	case *parser2.YieldFrom:
		b.expr(n.Value)
	case *parser2.Compare:
		b.expr(n.Left)
		for _, c := range n.Comparators {
			b.expr(c)
		}
	case *parser2.Call:
		b.expr(n.Func)
		for _, a := range n.Args {
			b.expr(a)
		}
		for _, kw := range n.Keywords {
			b.expr(kw.Value)
		}
	case *parser2.FormattedValue:
		b.expr(n.Value)
		b.expr(n.FormatSpec)
	case *parser2.Interpolation:
		b.expr(n.Value)
		b.expr(n.FormatSpec)
	case *parser2.JoinedStr:
		for _, v := range n.Values {
			b.expr(v)
		}
	case *parser2.TemplateStr:
		for _, interp := range n.Interpolations {
			b.expr(interp.Value)
			b.expr(interp.FormatSpec)
		}
	case *parser2.Attribute:
		b.expr(n.Value)
	case *parser2.Subscript:
		b.expr(n.Value)
		b.expr(n.Slice)
	case *parser2.Starred:
		b.expr(n.Value)
	case *parser2.Name:
		// In v2 all Names are Load context unless in target position;
		// target() handles the bind side, so here we always record a use.
		b.use(b.cur, n.Id, n.P)
	case *parser2.List:
		for _, v := range n.Elts {
			b.expr(v)
		}
	case *parser2.Tuple:
		for _, v := range n.Elts {
			b.expr(v)
		}
	case *parser2.Slice:
		b.expr(n.Lower)
		b.expr(n.Upper)
		b.expr(n.Step)
	}
}

func (b *builder) comp(pos parser2.Pos, gens []*parser2.Comprehension, elt func()) {
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

func (b *builder) augUse(e parser2.Expr) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *parser2.Name:
		b.use(b.cur, n.Id, n.P)
	case *parser2.Tuple:
		for _, x := range n.Elts {
			b.augUse(x)
		}
	case *parser2.List:
		for _, x := range n.Elts {
			b.augUse(x)
		}
	case *parser2.Starred:
		b.augUse(n.Value)
	}
}

func (b *builder) target(e parser2.Expr, flag BindFlag) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *parser2.Name:
		b.bind(b.cur, n.Id, n.P, flag)
	case *parser2.Tuple:
		for _, x := range n.Elts {
			b.target(x, flag)
		}
	case *parser2.List:
		for _, x := range n.Elts {
			b.target(x, flag)
		}
	case *parser2.Starred:
		b.target(n.Value, flag)
	case *parser2.Attribute:
		b.expr(n.Value)
	case *parser2.Subscript:
		b.expr(n.Value)
		b.expr(n.Slice)
	default:
		b.expr(e)
	}
}

func (b *builder) pattern(p parser2.Pattern) {
	if p == nil {
		return
	}
	switch n := p.(type) {
	case *parser2.MatchValue:
		b.expr(n.Value)
	case *parser2.MatchSingleton:
	case *parser2.MatchSequence:
		for _, q := range n.Patterns {
			b.pattern(q)
		}
	case *parser2.MatchMapping:
		for _, k := range n.Keys {
			b.expr(k)
		}
		for _, q := range n.Patterns {
			b.pattern(q)
		}
		if n.Rest != "" {
			b.bind(b.cur, n.Rest, n.P, FlagBound)
		}
	case *parser2.MatchClass:
		b.expr(n.Cls)
		for _, q := range n.Patterns {
			b.pattern(q)
		}
		for _, q := range n.KwdPatterns {
			b.pattern(q)
		}
	case *parser2.MatchStar:
		if n.Name != "" {
			b.bind(b.cur, n.Name, n.P, FlagBound)
		}
	case *parser2.MatchAs:
		b.pattern(n.Pattern)
		if n.Name != "" {
			b.bind(b.cur, n.Name, n.P, FlagBound)
		}
	case *parser2.MatchOr:
		for _, q := range n.Patterns {
			b.pattern(q)
		}
	}
}

func (b *builder) resolve(scope *Scope) {
	for _, child := range scope.Children {
		b.resolve(child)
	}
	if scope.Kind == ScopeModule || scope.Kind == ScopeClass {
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

func topImportName(dotted string) string {
	for i := 0; i < len(dotted); i++ {
		if dotted[i] == '.' {
			return dotted[:i]
		}
	}
	return dotted
}

// Resolve looks up name starting from scope, walking up the parent chain.
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
