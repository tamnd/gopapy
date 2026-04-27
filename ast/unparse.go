package ast

import (
	"bytes"
	"fmt"
	"math"
	"strings"
)

// Unparse renders an AST node back into Python source. The output round-trips
// through the parser to a structurally-equal AST; byte-equality with the
// original source is not a goal. Mirrors CPython's ast.unparse closely.
func Unparse(n Node) string {
	return UnparseWith(n, nil)
}

// UnparseHooks lets a higher-level caller inject text around each
// statement during Unparse. The cst package uses this to weave Trivia
// comments back into the output stream without copy-pasting the
// printer. A nil hook set degenerates to plain Unparse.
type UnparseHooks interface {
	// LeadingFor returns lines to print on their own, before s, at
	// s's indent level. The printer adds indentation and the trailing
	// newline; return the text only (e.g. "# comment").
	LeadingFor(s StmtNode) []string

	// TrailingFor returns the suffix to append on the same line as s's
	// final emitted newline, prefixed automatically with two spaces.
	// Return "" when there's nothing to append.
	TrailingFor(s StmtNode) string

	// FileTrailing returns lines to emit at module scope after the
	// last statement. Indented to zero. Called once after the body of
	// a *Module finishes.
	FileTrailing() []string
}

// UnparseWith is Unparse plus a hooks argument. Pass nil for the
// hooks to get plain Unparse output.
func UnparseWith(n Node, h UnparseHooks) string {
	p := &printer{hooks: h}
	p.node(n)
	return p.b.String()
}

type printer struct {
	b      bytes.Buffer
	indent int
	hooks  UnparseHooks
}

func (p *printer) write(s string) { p.b.WriteString(s) }

func (p *printer) writeIndent() {
	for i := 0; i < p.indent; i++ {
		p.b.WriteString("    ")
	}
}

func (p *printer) newline() { p.b.WriteByte('\n') }

// Operator precedence, matching CPython's _Unparser.
const (
	pTuple int = iota // top-level tuple position (no parens needed)
	pTest             // lambda / if-else
	pNamed
	pOr
	pAnd
	pNot
	pCmp
	pBitOr
	pBitXor
	pBitAnd
	pShift
	pArith
	pTerm
	pFactor
	pPower
	pAwait
	pAtom
)

func (p *printer) node(n Node) {
	switch v := n.(type) {
	case ModNode:
		p.mod(v)
	case StmtNode:
		p.stmt(v)
	case ExprNode:
		p.expr(v, pTest)
	case PatternNode:
		p.pattern(v)
	case ExcepthandlerNode:
		p.excHandler(v.(*ExceptHandler), false)
	case TypeParamNode:
		p.typeParam(v)
	default:
		fmt.Fprintf(&p.b, "<%T>", n)
	}
}

// ---------------------------------------------------------------------------
// Modules
// ---------------------------------------------------------------------------

func (p *printer) mod(m ModNode) {
	switch v := m.(type) {
	case *Module:
		p.body(v.Body)
		if p.hooks != nil {
			for _, line := range p.hooks.FileTrailing() {
				p.writeIndent()
				p.write(line)
				p.newline()
			}
		}
	case *Interactive:
		p.body(v.Body)
	case *Expression:
		p.expr(v.Body, pTest)
	case *FunctionType:
		p.write("(")
		for i, a := range v.Argtypes {
			if i > 0 {
				p.write(", ")
			}
			p.expr(a, pTest)
		}
		p.write(") -> ")
		p.expr(v.Returns, pTest)
	}
}

func (p *printer) body(stmts []StmtNode) {
	for _, s := range stmts {
		p.stmt(s)
	}
}

func (p *printer) suite(stmts []StmtNode) {
	p.write(":")
	p.newline()
	p.indent++
	if len(stmts) == 0 {
		p.writeIndent()
		p.write("pass")
		p.newline()
	} else {
		for _, s := range stmts {
			p.stmt(s)
		}
	}
	p.indent--
}

// ---------------------------------------------------------------------------
// Statements
// ---------------------------------------------------------------------------

func (p *printer) stmt(s StmtNode) {
	if p.hooks != nil {
		for _, line := range p.hooks.LeadingFor(s) {
			p.writeIndent()
			p.write(line)
			p.newline()
		}
	}
	p.stmtBody(s)
	if p.hooks != nil {
		if t := p.hooks.TrailingFor(s); t != "" {
			p.appendTrailing(t)
		}
	}
}

// appendTrailing rewrites the statement's terminating newline so the
// trailing comment lands on the same line: ``stmt\n`` becomes
// ``stmt  # comment\n``. If the statement didn't end with a newline
// (shouldn't happen for valid trees, but stay defensive), the comment
// just gets appended with a single space separator.
func (p *printer) appendTrailing(t string) {
	buf := p.b.Bytes()
	if n := len(buf); n > 0 && buf[n-1] == '\n' {
		p.b.Truncate(n - 1)
		p.write("  ")
		p.write(t)
		p.newline()
		return
	}
	p.write(" ")
	p.write(t)
}

func (p *printer) stmtBody(s StmtNode) {
	switch v := s.(type) {
	case *FunctionDef:
		p.funcDef(v.Name, v.Args, v.Body, v.DecoratorList, v.Returns, v.TypeParams, false)
	case *AsyncFunctionDef:
		p.funcDef(v.Name, v.Args, v.Body, v.DecoratorList, v.Returns, v.TypeParams, true)
	case *ClassDef:
		p.classDef(v)
	case *Return:
		p.writeIndent()
		p.write("return")
		if v.Value != nil {
			p.write(" ")
			p.expr(v.Value, pTuple)
		}
		p.newline()
	case *Delete:
		p.writeIndent()
		p.write("del ")
		for i, t := range v.Targets {
			if i > 0 {
				p.write(", ")
			}
			p.expr(t, pTest)
		}
		p.newline()
	case *Assign:
		p.writeIndent()
		for _, t := range v.Targets {
			p.expr(t, pTuple)
			p.write(" = ")
		}
		p.expr(v.Value, pTuple)
		p.newline()
	case *TypeAlias:
		p.writeIndent()
		p.write("type ")
		p.expr(v.Name, pAtom)
		p.typeParamList(v.TypeParams)
		p.write(" = ")
		p.expr(v.Value, pTest)
		p.newline()
	case *AugAssign:
		p.writeIndent()
		p.expr(v.Target, pTest)
		p.write(" ")
		p.write(opSym(v.Op))
		p.write("= ")
		// A bare `*x` element inside a tuple isn't accepted on the RHS of
		// AugAssign without surrounding parens, so promote precedence to
		// force the Tuple emitter to wrap.
		valPrec := pTuple
		if t, ok := v.Value.(*Tuple); ok && tupleHasStarred(t) {
			valPrec = pAtom
		}
		p.expr(v.Value, valPrec)
		p.newline()
	case *AnnAssign:
		p.writeIndent()
		if v.Simple == 0 {
			p.write("(")
			p.expr(v.Target, pTest)
			p.write(")")
		} else {
			p.expr(v.Target, pTest)
		}
		p.write(": ")
		p.expr(v.Annotation, pTest)
		if v.Value != nil {
			p.write(" = ")
			p.expr(v.Value, pTest)
		}
		p.newline()
	case *For:
		p.forStmt(v.Target, v.Iter, v.Body, v.Orelse, false)
	case *AsyncFor:
		p.forStmt(v.Target, v.Iter, v.Body, v.Orelse, true)
	case *While:
		p.writeIndent()
		p.write("while ")
		p.expr(v.Test, pTest)
		p.suite(v.Body)
		if len(v.Orelse) > 0 {
			p.writeIndent()
			p.write("else")
			p.suite(v.Orelse)
		}
	case *If:
		p.ifStmt(v, false)
	case *With:
		p.withStmt(v.Items, v.Body, false)
	case *AsyncWith:
		p.withStmt(v.Items, v.Body, true)
	case *Match:
		p.matchStmt(v)
	case *Raise:
		p.writeIndent()
		p.write("raise")
		if v.Exc != nil {
			p.write(" ")
			p.expr(v.Exc, pTest)
		}
		if v.Cause != nil {
			p.write(" from ")
			p.expr(v.Cause, pTest)
		}
		p.newline()
	case *Try:
		p.tryStmt(v.Body, v.Handlers, v.Orelse, v.Finalbody, false)
	case *TryStar:
		p.tryStmt(v.Body, v.Handlers, v.Orelse, v.Finalbody, true)
	case *Assert:
		p.writeIndent()
		p.write("assert ")
		p.expr(v.Test, pTest)
		if v.Msg != nil {
			p.write(", ")
			p.expr(v.Msg, pTest)
		}
		p.newline()
	case *Import:
		p.writeIndent()
		p.write("import ")
		for i, a := range v.Names {
			if i > 0 {
				p.write(", ")
			}
			p.alias(a)
		}
		p.newline()
	case *ImportFrom:
		p.writeIndent()
		p.write("from ")
		for i := 0; i < v.Level; i++ {
			p.write(".")
		}
		p.write(v.Module)
		p.write(" import ")
		if len(v.Names) == 1 && v.Names[0].Name == "*" {
			p.write("*")
		} else {
			for i, a := range v.Names {
				if i > 0 {
					p.write(", ")
				}
				p.alias(a)
			}
		}
		p.newline()
	case *Global:
		p.writeIndent()
		p.write("global ")
		p.write(strings.Join(v.Names, ", "))
		p.newline()
	case *Nonlocal:
		p.writeIndent()
		p.write("nonlocal ")
		p.write(strings.Join(v.Names, ", "))
		p.newline()
	case *Expr:
		p.writeIndent()
		// A bare yield/yield-from at statement level renders without the
		// surrounding parens that the expression form requires.
		switch y := v.Value.(type) {
		case *Yield:
			p.write("yield")
			if y.Value != nil {
				p.write(" ")
				p.expr(y.Value, pTuple)
			}
		case *YieldFrom:
			p.write("yield from ")
			p.expr(y.Value, pTest)
		default:
			p.expr(v.Value, pTuple)
		}
		p.newline()
	case *Pass:
		p.writeIndent()
		p.write("pass")
		p.newline()
	case *Break:
		p.writeIndent()
		p.write("break")
		p.newline()
	case *Continue:
		p.writeIndent()
		p.write("continue")
		p.newline()
	}
}

func (p *printer) alias(a *Alias) {
	p.write(a.Name)
	if a.Asname != "" {
		p.write(" as ")
		p.write(a.Asname)
	}
}

func (p *printer) decorators(decos []ExprNode) {
	for _, d := range decos {
		p.writeIndent()
		p.write("@")
		p.expr(d, pTest)
		p.newline()
	}
}

func (p *printer) funcDef(name string, args *Arguments, body []StmtNode, decos []ExprNode, returns ExprNode, tps []TypeParamNode, async bool) {
	p.decorators(decos)
	p.writeIndent()
	if async {
		p.write("async ")
	}
	p.write("def ")
	p.write(name)
	p.typeParamList(tps)
	p.write("(")
	p.arguments(args)
	p.write(")")
	if returns != nil {
		p.write(" -> ")
		p.expr(returns, pTest)
	}
	p.suite(body)
}

func (p *printer) classDef(c *ClassDef) {
	p.decorators(c.DecoratorList)
	p.writeIndent()
	p.write("class ")
	p.write(c.Name)
	p.typeParamList(c.TypeParams)
	if len(c.Bases) > 0 || len(c.Keywords) > 0 {
		p.write("(")
		first := true
		for _, b := range c.Bases {
			if !first {
				p.write(", ")
			}
			first = false
			p.expr(b, pTest)
		}
		for _, k := range c.Keywords {
			if !first {
				p.write(", ")
			}
			first = false
			p.keyword(k)
		}
		p.write(")")
	}
	p.suite(c.Body)
}

func (p *printer) ifStmt(v *If, isElif bool) {
	p.writeIndent()
	if isElif {
		p.write("elif ")
	} else {
		p.write("if ")
	}
	p.expr(v.Test, pTest)
	p.suite(v.Body)
	if len(v.Orelse) == 1 {
		if elif, ok := v.Orelse[0].(*If); ok {
			p.ifStmt(elif, true)
			return
		}
	}
	if len(v.Orelse) > 0 {
		p.writeIndent()
		p.write("else")
		p.suite(v.Orelse)
	}
}

func (p *printer) forStmt(target, iter ExprNode, body, orelse []StmtNode, async bool) {
	p.writeIndent()
	if async {
		p.write("async ")
	}
	p.write("for ")
	p.expr(target, pTuple)
	p.write(" in ")
	p.expr(iter, pTuple)
	p.suite(body)
	if len(orelse) > 0 {
		p.writeIndent()
		p.write("else")
		p.suite(orelse)
	}
}

func (p *printer) withStmt(items []*Withitem, body []StmtNode, async bool) {
	p.writeIndent()
	if async {
		p.write("async ")
	}
	p.write("with ")
	for i, it := range items {
		if i > 0 {
			p.write(", ")
		}
		p.expr(it.ContextExpr, pTest)
		if it.OptionalVars != nil {
			p.write(" as ")
			p.expr(it.OptionalVars, pTest)
		}
	}
	p.suite(body)
}

func (p *printer) tryStmt(body []StmtNode, handlers []ExcepthandlerNode, orelse, final []StmtNode, star bool) {
	p.writeIndent()
	p.write("try")
	p.suite(body)
	for _, h := range handlers {
		p.excHandler(h.(*ExceptHandler), star)
	}
	if len(orelse) > 0 {
		p.writeIndent()
		p.write("else")
		p.suite(orelse)
	}
	if len(final) > 0 {
		p.writeIndent()
		p.write("finally")
		p.suite(final)
	}
}

func (p *printer) excHandler(h *ExceptHandler, star bool) {
	p.writeIndent()
	if star {
		p.write("except*")
	} else {
		p.write("except")
	}
	if h.Type != nil {
		p.write(" ")
		p.expr(h.Type, pTest)
		if h.Name != "" {
			p.write(" as ")
			p.write(h.Name)
		}
	}
	p.suite(h.Body)
}

// ---------------------------------------------------------------------------
// Match
// ---------------------------------------------------------------------------

func (p *printer) matchStmt(m *Match) {
	p.writeIndent()
	p.write("match ")
	p.matchSubject(m.Subject)
	p.write(":")
	p.newline()
	p.indent++
	for _, c := range m.Cases {
		p.writeIndent()
		p.write("case ")
		p.pattern(c.Pattern)
		if c.Guard != nil {
			p.write(" if ")
			p.expr(c.Guard, pTest)
		}
		p.suite(c.Body)
	}
	p.indent--
}

func (p *printer) matchSubject(e ExprNode) {
	if t, ok := e.(*Tuple); ok {
		// Empty and single-element tuples need parens: a bare `match :`
		// or `match x,:` re-parses as something else (or nothing). The
		// multi-element case can drop the parens since `match a, b:` is
		// the canonical form.
		if len(t.Elts) == 0 {
			p.write("()")
			return
		}
		if len(t.Elts) == 1 {
			p.write("(")
			p.expr(t.Elts[0], pTest)
			p.write(",)")
			return
		}
		for i, el := range t.Elts {
			if i > 0 {
				p.write(", ")
			}
			p.expr(el, pTest)
		}
		return
	}
	p.expr(e, pTest)
}

func (p *printer) pattern(pn PatternNode) {
	switch v := pn.(type) {
	case *MatchValue:
		p.matchValueExpr(v.Value)
	case *MatchSingleton:
		p.constant(v.Value, "")
	case *MatchSequence:
		p.write("[")
		for i, sub := range v.Patterns {
			if i > 0 {
				p.write(", ")
			}
			p.pattern(sub)
		}
		p.write("]")
	case *MatchMapping:
		p.write("{")
		first := true
		for i, k := range v.Keys {
			if !first {
				p.write(", ")
			}
			first = false
			p.expr(k, pTest)
			p.write(": ")
			p.pattern(v.Patterns[i])
		}
		if v.Rest != "" {
			if !first {
				p.write(", ")
			}
			p.write("**")
			p.write(v.Rest)
		}
		p.write("}")
	case *MatchClass:
		p.expr(v.Cls, pAtom)
		p.write("(")
		first := true
		for _, sub := range v.Patterns {
			if !first {
				p.write(", ")
			}
			first = false
			p.pattern(sub)
		}
		for i, attr := range v.KwdAttrs {
			if !first {
				p.write(", ")
			}
			first = false
			p.write(attr)
			p.write("=")
			p.pattern(v.KwdPatterns[i])
		}
		p.write(")")
	case *MatchStar:
		p.write("*")
		if v.Name != "" {
			p.write(v.Name)
		} else {
			p.write("_")
		}
	case *MatchAs:
		if v.Pattern == nil && v.Name == "" {
			p.write("_")
			return
		}
		if v.Pattern == nil {
			p.write(v.Name)
			return
		}
		// Nested `as` needs parens: `0 as w as z` re-parses as a syntax
		// error; the source form is `(0 as w) as z`.
		if _, ok := v.Pattern.(*MatchAs); ok {
			p.write("(")
			p.pattern(v.Pattern)
			p.write(")")
		} else {
			p.pattern(v.Pattern)
		}
		p.write(" as ")
		p.write(v.Name)
	case *MatchOr:
		for i, sub := range v.Patterns {
			if i > 0 {
				p.write(" | ")
			}
			// `as` binds looser than `|`, so a bare `0 as z | 1 as z`
			// re-parses as MatchAs(MatchOr(...)). And a nested MatchOr
			// would otherwise flatten on reparse, losing structure.
			// Wrap both in parens to preserve the original tree.
			switch sub.(type) {
			case *MatchAs, *MatchOr:
				p.write("(")
				p.pattern(sub)
				p.write(")")
				continue
			}
			p.pattern(sub)
		}
	}
}

// ---------------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------------

func (p *printer) expr(e ExprNode, prec int) {
	switch v := e.(type) {
	case *BoolOp:
		p.boolOp(v, prec)
	case *NamedExpr:
		paren := prec > pNamed
		if paren {
			p.write("(")
		}
		p.expr(v.Target, pAtom)
		p.write(" := ")
		p.expr(v.Value, pNamed+1)
		if paren {
			p.write(")")
		}
	case *BinOp:
		p.binOp(v, prec)
	case *UnaryOp:
		p.unaryOp(v, prec)
	case *Lambda:
		paren := prec > pTest
		if paren {
			p.write("(")
		}
		p.write("lambda")
		if v.Args != nil && argumentsHasAny(v.Args) {
			p.write(" ")
			p.arguments(v.Args)
		}
		p.write(": ")
		p.expr(v.Body, pTest)
		if paren {
			p.write(")")
		}
	case *IfExp:
		paren := prec > pTest
		if paren {
			p.write("(")
		}
		p.expr(v.Body, pTest+1)
		p.write(" if ")
		p.expr(v.Test, pTest+1)
		p.write(" else ")
		p.expr(v.Orelse, pTest)
		if paren {
			p.write(")")
		}
	case *Dict:
		p.write("{")
		for i := range v.Keys {
			if i > 0 {
				p.write(", ")
			}
			if v.Keys[i] == nil {
				p.write("**")
				p.expr(v.Values[i], pBitOr)
			} else {
				p.expr(v.Keys[i], pTest)
				p.write(": ")
				p.expr(v.Values[i], pTest)
			}
		}
		p.write("}")
	case *Set:
		if len(v.Elts) == 0 {
			p.write("set()")
			return
		}
		p.write("{")
		for i, el := range v.Elts {
			if i > 0 {
				p.write(", ")
			}
			p.expr(el, pTest)
		}
		p.write("}")
	case *ListComp:
		p.write("[")
		p.expr(v.Elt, pTest)
		p.comprehensions(v.Generators)
		p.write("]")
	case *SetComp:
		p.write("{")
		p.expr(v.Elt, pTest)
		p.comprehensions(v.Generators)
		p.write("}")
	case *DictComp:
		p.write("{")
		p.expr(v.Key, pTest)
		p.write(": ")
		p.expr(v.Value, pTest)
		p.comprehensions(v.Generators)
		p.write("}")
	case *GeneratorExp:
		p.write("(")
		p.expr(v.Elt, pTest)
		p.comprehensions(v.Generators)
		p.write(")")
	case *Await:
		paren := prec > pAwait
		if paren {
			p.write("(")
		}
		p.write("await ")
		// `await await x` is a syntax error; the inner await needs parens.
		if _, inner := v.Value.(*Await); inner {
			p.write("(")
			p.expr(v.Value, 0)
			p.write(")")
		} else {
			p.expr(v.Value, pAwait)
		}
		if paren {
			p.write(")")
		}
	case *Yield:
		p.write("(")
		p.write("yield")
		if v.Value != nil {
			p.write(" ")
			p.expr(v.Value, pTuple)
		}
		p.write(")")
	case *YieldFrom:
		p.write("(yield from ")
		p.expr(v.Value, pTest)
		p.write(")")
	case *Compare:
		p.compare(v, prec)
	case *Call:
		p.call(v)
	case *FormattedValue:
		p.write("f")
		p.fstringOuter(func() { p.formattedValueInner(v) })
	case *JoinedStr:
		p.write("f")
		p.fstringOuter(func() { p.joinedStrInner(v.Values) })
	case *Interpolation:
		p.write("t")
		p.fstringOuter(func() { p.interpolationInner(v) })
	case *TemplateStr:
		p.write("t")
		p.fstringOuter(func() { p.templateStrInner(v.Values) })
	case *Constant:
		p.constant(v.Value, v.Kind)
	case *Attribute:
		p.attribute(v)
	case *Subscript:
		p.expr(v.Value, pAtom)
		p.write("[")
		p.subscriptSlice(v.Slice)
		p.write("]")
	case *Starred:
		p.write("*")
		p.expr(v.Value, pAtom)
	case *Name:
		p.write(v.Id)
	case *List:
		p.write("[")
		for i, el := range v.Elts {
			if i > 0 {
				p.write(", ")
			}
			p.expr(el, pTest)
		}
		p.write("]")
	case *Tuple:
		// Empty tuple needs parens.
		if len(v.Elts) == 0 {
			p.write("()")
			return
		}
		// Single-element tuples need parens to disambiguate the trailing
		// comma — otherwise contexts that strip the comma (e.g.
		// AugAssign right-hand side in our parser, but also several
		// CPython-conventional places) silently demote it to a scalar.
		// CPython's ast.unparse takes the same line.
		paren := prec > pTuple || len(v.Elts) == 1
		if paren {
			p.write("(")
		}
		for i, el := range v.Elts {
			if i > 0 {
				p.write(", ")
			}
			p.expr(el, pTest)
		}
		if len(v.Elts) == 1 {
			p.write(",")
		}
		if paren {
			p.write(")")
		}
	case *Slice:
		p.sliceExpr(v)
	}
}

// matchValueExpr emits a value pattern. The match-pattern grammar only allows
// `signed_number (+|-) NUMBER` for complex literals — the imaginary side must
// be a bare positive numeric literal. Our parser folds `0 - 0j` into
// `BinOp(Add, 0, Constant(-0j))`, which would re-emit as the ungrammatical
// `0 + -0.0j`. Detect a negative numeric right-hand constant and flip the
// operator so the output stays parseable.
func (p *printer) matchValueExpr(e ExprNode) {
	if b, ok := e.(*BinOp); ok {
		_, isAdd := b.Op.(*Add)
		_, isSub := b.Op.(*Sub)
		if isAdd || isSub {
			if c, ok := b.Right.(*Constant); ok {
				if neg, abs := negateIfNegativeNumeric(c.Value); neg {
					p.expr(b.Left, pTest)
					if isAdd {
						p.write(" - ")
					} else {
						p.write(" + ")
					}
					p.constant(abs, c.Kind)
					return
				}
			}
		}
	}
	p.expr(e, pTest)
}

// negateIfNegativeNumeric returns (true, abs) when v is a negative numeric
// constant (float or complex with negative imaginary). Otherwise (false, v).
// Int constants are stored as decimal strings without a leading `-` (the
// negation is in a UnaryOp), so they never appear here.
func negateIfNegativeNumeric(v ConstantValue) (bool, ConstantValue) {
	switch v.Kind {
	case ConstantFloat:
		if v.Float < 0 || (v.Float == 0 && math.Signbit(v.Float)) {
			n := v
			n.Float = -v.Float
			return true, n
		}
	case ConstantComplex:
		if v.Imag < 0 || (v.Imag == 0 && math.Signbit(v.Imag)) {
			n := v
			n.Imag = -v.Imag
			return true, n
		}
	}
	return false, v
}

func tupleHasStarred(t *Tuple) bool {
	for _, el := range t.Elts {
		if _, ok := el.(*Starred); ok {
			return true
		}
	}
	return false
}

func argumentsHasAny(a *Arguments) bool {
	return len(a.Args) > 0 || len(a.Posonlyargs) > 0 || len(a.Kwonlyargs) > 0 || a.Vararg != nil || a.Kwarg != nil
}

func (p *printer) attribute(a *Attribute) {
	// Bare integer attribute (e.g. 1 .imag) needs a space; we wrap in parens.
	if c, ok := a.Value.(*Constant); ok && c.Value.Kind == ConstantInt {
		p.write("(")
		p.expr(a.Value, pAtom)
		p.write(")")
	} else {
		p.expr(a.Value, pAtom)
	}
	p.write(".")
	p.write(a.Attr)
}

func (p *printer) subscriptSlice(s ExprNode) {
	if t, ok := s.(*Tuple); ok && len(t.Elts) > 0 {
		// Single-element tuples must keep their parens here too: a bare
		// `1,` inside `[]` re-parses as the scalar 1, not a 1-tuple,
		// because the parser drops trailing commas in subscript context.
		if len(t.Elts) == 1 {
			p.write("(")
			p.expr(t.Elts[0], pTest)
			p.write(",)")
			return
		}
		for i, el := range t.Elts {
			if i > 0 {
				p.write(", ")
			}
			p.expr(el, pTest)
		}
		return
	}
	p.expr(s, pTest)
}

func (p *printer) sliceExpr(s *Slice) {
	if s.Lower != nil {
		p.expr(s.Lower, pTest)
	}
	p.write(":")
	if s.Upper != nil {
		p.expr(s.Upper, pTest)
	}
	if s.Step != nil {
		p.write(":")
		p.expr(s.Step, pTest)
	}
}

func (p *printer) call(c *Call) {
	p.expr(c.Func, pAtom)
	p.write("(")
	first := true
	for _, a := range c.Args {
		if !first {
			p.write(", ")
		}
		first = false
		p.expr(a, pTest)
	}
	for _, k := range c.Keywords {
		if !first {
			p.write(", ")
		}
		first = false
		p.keyword(k)
	}
	p.write(")")
}

func (p *printer) keyword(k *Keyword) {
	if k.Arg == "" {
		p.write("**")
		p.expr(k.Value, pTest)
		return
	}
	p.write(k.Arg)
	p.write("=")
	p.expr(k.Value, pTest)
}

func (p *printer) comprehensions(comps []*Comprehension) {
	for _, c := range comps {
		if c.IsAsync != 0 {
			p.write(" async for ")
		} else {
			p.write(" for ")
		}
		p.expr(c.Target, pTuple)
		p.write(" in ")
		p.expr(c.Iter, pOr)
		for _, f := range c.Ifs {
			p.write(" if ")
			p.expr(f, pOr)
		}
	}
}

func (p *printer) compare(c *Compare, prec int) {
	paren := prec > pCmp
	if paren {
		p.write("(")
	}
	p.expr(c.Left, pCmp+1)
	for i, op := range c.Ops {
		p.write(" ")
		p.write(cmpSym(op))
		p.write(" ")
		p.expr(c.Comparators[i], pCmp+1)
	}
	if paren {
		p.write(")")
	}
}

func (p *printer) boolOp(b *BoolOp, prec int) {
	op := pOr
	sym := " or "
	if _, ok := b.Op.(*And); ok {
		op = pAnd
		sym = " and "
	}
	paren := prec > op
	if paren {
		p.write("(")
	}
	for i, v := range b.Values {
		if i > 0 {
			p.write(sym)
		}
		p.expr(v, op+1)
	}
	if paren {
		p.write(")")
	}
}

func (p *printer) binOp(b *BinOp, prec int) {
	myPrec := opPrec(b.Op)
	paren := prec > myPrec
	if paren {
		p.write("(")
	}
	// Power is right-associative.
	left, right := myPrec, myPrec+1
	if _, ok := b.Op.(*Pow); ok {
		left, right = myPrec+1, myPrec
	}
	p.expr(b.Left, left)
	sym := opSym(b.Op)
	if _, ok := b.Op.(*Pow); ok {
		p.write(sym)
	} else {
		p.write(" ")
		p.write(sym)
		p.write(" ")
	}
	p.expr(b.Right, right)
	if paren {
		p.write(")")
	}
}

func (p *printer) unaryOp(u *UnaryOp, prec int) {
	if _, ok := u.Op.(*Not); ok {
		paren := prec > pNot
		if paren {
			p.write("(")
		}
		p.write("not ")
		p.expr(u.Operand, pNot)
		if paren {
			p.write(")")
		}
		return
	}
	paren := prec > pFactor
	if paren {
		p.write("(")
	}
	switch u.Op.(type) {
	case *UAdd:
		p.write("+")
	case *USub:
		p.write("-")
	case *Invert:
		p.write("~")
	}
	p.expr(u.Operand, pFactor)
	if paren {
		p.write(")")
	}
}

func opPrec(op OperatorNode) int {
	switch op.(type) {
	case *BitOr:
		return pBitOr
	case *BitXor:
		return pBitXor
	case *BitAnd:
		return pBitAnd
	case *LShift, *RShift:
		return pShift
	case *Add, *Sub:
		return pArith
	case *Mult, *Div, *FloorDiv, *Mod, *MatMult:
		return pTerm
	case *Pow:
		return pPower
	}
	return pAtom
}

func opSym(op OperatorNode) string {
	switch op.(type) {
	case *Add:
		return "+"
	case *Sub:
		return "-"
	case *Mult:
		return "*"
	case *MatMult:
		return "@"
	case *Div:
		return "/"
	case *Mod:
		return "%"
	case *Pow:
		return "**"
	case *LShift:
		return "<<"
	case *RShift:
		return ">>"
	case *BitOr:
		return "|"
	case *BitXor:
		return "^"
	case *BitAnd:
		return "&"
	case *FloorDiv:
		return "//"
	}
	return "?"
}

func cmpSym(op CmpopNode) string {
	switch op.(type) {
	case *Eq:
		return "=="
	case *NotEq:
		return "!="
	case *Lt:
		return "<"
	case *LtE:
		return "<="
	case *Gt:
		return ">"
	case *GtE:
		return ">="
	case *Is:
		return "is"
	case *IsNot:
		return "is not"
	case *In:
		return "in"
	case *NotIn:
		return "not in"
	}
	return "?"
}

// ---------------------------------------------------------------------------
// Arguments
// ---------------------------------------------------------------------------

func (p *printer) arguments(a *Arguments) {
	if a == nil {
		return
	}
	first := true
	emit := func(s string) {
		if !first {
			p.write(", ")
		}
		first = false
		p.write(s)
	}
	emitArg := func(arg *Arg, def ExprNode) {
		if !first {
			p.write(", ")
		}
		first = false
		p.write(arg.Arg)
		if arg.Annotation != nil {
			p.write(": ")
			p.expr(arg.Annotation, pTest)
		}
		if def != nil {
			if arg.Annotation != nil {
				p.write(" = ")
			} else {
				p.write("=")
			}
			p.expr(def, pTest)
		}
	}

	// Compute defaults alignment: trailing run of positional defaults.
	posCount := len(a.Posonlyargs) + len(a.Args)
	defOffset := posCount - len(a.Defaults)
	getDef := func(idx int) ExprNode {
		if idx < defOffset {
			return nil
		}
		return a.Defaults[idx-defOffset]
	}

	idx := 0
	for _, arg := range a.Posonlyargs {
		emitArg(arg, getDef(idx))
		idx++
	}
	if len(a.Posonlyargs) > 0 {
		emit("/")
	}
	for _, arg := range a.Args {
		emitArg(arg, getDef(idx))
		idx++
	}

	if a.Vararg != nil {
		if !first {
			p.write(", ")
		}
		first = false
		p.write("*")
		p.write(a.Vararg.Arg)
		if a.Vararg.Annotation != nil {
			p.write(": ")
			p.expr(a.Vararg.Annotation, pTest)
		}
	} else if len(a.Kwonlyargs) > 0 {
		emit("*")
	}

	for i, arg := range a.Kwonlyargs {
		var def ExprNode
		if i < len(a.KwDefaults) {
			def = a.KwDefaults[i]
		}
		emitArg(arg, def)
	}

	if a.Kwarg != nil {
		if !first {
			p.write(", ")
		}
		first = false
		p.write("**")
		p.write(a.Kwarg.Arg)
		if a.Kwarg.Annotation != nil {
			p.write(": ")
			p.expr(a.Kwarg.Annotation, pTest)
		}
	}
}

// ---------------------------------------------------------------------------
// Type parameters
// ---------------------------------------------------------------------------

func (p *printer) typeParamList(tps []TypeParamNode) {
	if len(tps) == 0 {
		return
	}
	p.write("[")
	for i, tp := range tps {
		if i > 0 {
			p.write(", ")
		}
		p.typeParam(tp)
	}
	p.write("]")
}

func (p *printer) typeParam(tp TypeParamNode) {
	switch v := tp.(type) {
	case *TypeVar:
		p.write(v.Name)
		if v.Bound != nil {
			p.write(": ")
			p.expr(v.Bound, pTest)
		}
		if v.DefaultValue != nil {
			p.write(" = ")
			p.expr(v.DefaultValue, pTest)
		}
	case *ParamSpec:
		p.write("**")
		p.write(v.Name)
		if v.DefaultValue != nil {
			p.write(" = ")
			p.expr(v.DefaultValue, pTest)
		}
	case *TypeVarTuple:
		p.write("*")
		p.write(v.Name)
		if v.DefaultValue != nil {
			p.write(" = ")
			p.expr(v.DefaultValue, pTest)
		}
	}
}

// ---------------------------------------------------------------------------
// Constants and strings
// ---------------------------------------------------------------------------

func (p *printer) constant(c ConstantValue, kind string) {
	switch c.Kind {
	case ConstantNone:
		p.write("None")
	case ConstantBool:
		if c.Bool {
			p.write("True")
		} else {
			p.write("False")
		}
	case ConstantInt:
		p.write(c.Int)
	case ConstantFloat:
		p.write(unparseFloatRepr(c.Float))
	case ConstantComplex:
		p.write(unparseFloatRepr(c.Imag))
		p.write("j")
	case ConstantStr:
		if kind == "u" {
			p.write("u")
		}
		p.write(pyStringLiteral(c.Str))
	case ConstantBytes:
		p.write("b")
		p.write(pyBytesLiteral(c.Bytes))
	case ConstantEllipsis:
		p.write("...")
	}
}

// unparseFloatRepr renders a float for output that must re-parse. Python
// has no `inf`/`nan` literals — pyFloatRepr emits those for ast.Dump
// (matching CPython repr) but they round-trip to a Name. CPython's
// ast.unparse substitutes `1e309` (overflows to inf) and `(1e309-1e309)`
// (NaN). The same trick handles complex constants: `1e309j`.
func unparseFloatRepr(f float64) string {
	if math.IsNaN(f) {
		return "(1e309-1e309)"
	}
	if math.IsInf(f, 1) {
		return "1e309"
	}
	if math.IsInf(f, -1) {
		return "-1e309"
	}
	return pyFloatRepr(f)
}

// pyStringLiteral renders a Go string as a Python string literal. Prefer
// single quotes; switch to double if the body contains a single quote and
// no double quote; escape as a last resort.
func pyStringLiteral(s string) string {
	hasSingle := strings.ContainsRune(s, '\'')
	hasDouble := strings.ContainsRune(s, '"')
	quote := byte('\'')
	if hasSingle && !hasDouble {
		quote = '"'
	}
	return encodeStringWithQuote(s, quote)
}

func pyBytesLiteral(buf []byte) string {
	hasSingle := false
	hasDouble := false
	for _, c := range buf {
		if c == '\'' {
			hasSingle = true
		}
		if c == '"' {
			hasDouble = true
		}
	}
	quote := byte('\'')
	if hasSingle && !hasDouble {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, c := range buf {
		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c == quote:
			b.WriteByte('\\')
			b.WriteByte(quote)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < 0x20 || c >= 0x7f:
			fmt.Fprintf(&b, `\x%02x`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func encodeStringWithQuote(s string, quote byte) string {
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case rune(quote):
			b.WriteByte('\\')
			b.WriteByte(quote)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// ---------------------------------------------------------------------------
// F-strings and t-strings
// ---------------------------------------------------------------------------

// fstringOuter wraps the body emitter with quotes. We always use double
// quotes for f-strings to keep the inner-expression strings simple (single
// quotes inside expressions are common).
func (p *printer) fstringOuter(emitBody func()) {
	p.write("\"")
	emitBody()
	p.write("\"")
}

func (p *printer) joinedStrInner(values []ExprNode) {
	for _, v := range values {
		switch x := v.(type) {
		case *Constant:
			if x.Value.Kind == ConstantStr {
				p.write(escapeFStringLiteral(x.Value.Str))
			}
		case *FormattedValue:
			p.formattedValueInner(x)
		default:
			// Defensive fallback.
			p.write("{")
			p.write(unparseInline(v))
			p.write("}")
		}
	}
}

func (p *printer) templateStrInner(values []ExprNode) {
	for _, v := range values {
		switch x := v.(type) {
		case *Constant:
			if x.Value.Kind == ConstantStr {
				p.write(escapeFStringLiteral(x.Value.Str))
			}
		case *Interpolation:
			p.interpolationInner(x)
		default:
			p.write("{")
			p.write(unparseInline(v))
			p.write("}")
		}
	}
}

func (p *printer) formattedValueInner(fv *FormattedValue) {
	p.write("{")
	body := unparseInline(fv.Value)
	if len(body) > 0 && body[0] == '{' {
		// Pad with a space so the leading `{` isn't lexed as `{{` escape.
		p.write(" ")
	}
	p.write(body)
	if len(body) > 0 && body[len(body)-1] == '}' && fv.FormatSpec == nil && fv.Conversion <= 0 {
		// Same trailing-space rule for `}` so `}}` doesn't escape.
		p.write(" ")
	}
	if fv.Conversion > 0 {
		p.write("!")
		p.write(string(rune(fv.Conversion)))
	}
	if fv.FormatSpec != nil {
		p.write(":")
		if js, ok := fv.FormatSpec.(*JoinedStr); ok {
			p.fspecBody(js.Values)
		}
	}
	p.write("}")
}

func (p *printer) interpolationInner(it *Interpolation) {
	p.write("{")
	body := unparseInline(it.Value)
	if len(body) > 0 && body[0] == '{' {
		p.write(" ")
	}
	p.write(body)
	if len(body) > 0 && body[len(body)-1] == '}' && it.FormatSpec == nil && it.Conversion <= 0 {
		p.write(" ")
	}
	if it.Conversion > 0 {
		p.write("!")
		p.write(string(rune(it.Conversion)))
	}
	if it.FormatSpec != nil {
		p.write(":")
		if js, ok := it.FormatSpec.(*JoinedStr); ok {
			p.fspecBody(js.Values)
		}
	}
	p.write("}")
}

func (p *printer) fspecBody(values []ExprNode) {
	for _, v := range values {
		switch x := v.(type) {
		case *Constant:
			if x.Value.Kind == ConstantStr {
				p.write(escapeFStringLiteral(x.Value.Str))
			}
		case *FormattedValue:
			p.formattedValueInner(x)
		}
	}
}

// escapeFStringLiteral escapes raw text for an f-string body: braces
// double, the outer quote escapes, backslashes/newlines stay readable.
func escapeFStringLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '{':
			b.WriteString("{{")
		case '}':
			b.WriteString("}}")
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '"':
			b.WriteString(`\"`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// unparseInline produces the inline source for an expression embedded in
// an f-string interpolation. Newlines are replaced with spaces.
func unparseInline(e ExprNode) string {
	s := Unparse(e)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

