package ast

import (
	"strconv"
	"strings"

	plexer "github.com/alecthomas/participle/v2/lexer"

	"github.com/tamnd/gopapy/parser"
)

// FromFile converts a participle parse tree into the canonical Python AST.
// The returned Module mirrors the shape ast.parse() produces in CPython.
//
// Coverage tracks the bootstrap grammar exactly: every Statement and every
// expression precedence level documented in parser/grammar*.go, plus the
// common subscript/call/attribute trailers. Things that the bootstrap
// grammar deliberately leaves out (full slice corner cases, comprehensions,
// f-strings, pattern matching, decorators, type parameters) are
// emitter-side TODOs that show up as either nil fields or panics so PR2
// can find them by running the test corpus.
func FromFile(f *parser.File) *Module {
	body := make([]StmtNode, 0, len(f.Statements))
	for _, st := range f.Statements {
		body = append(body, emitStatements(st)...)
	}
	return &Module{Body: body}
}

func emitStatements(st *parser.Statement) []StmtNode {
	switch {
	case st.Compound != nil:
		return []StmtNode{emitCompound(st.Compound)}
	case st.Simples != nil:
		return emitSimples(st.Simples)
	}
	return nil
}

func emitSimples(s *parser.SimpleStmts) []StmtNode {
	out := make([]StmtNode, 0, 1+len(s.Rest))
	out = append(out, emitSimple(s.First))
	for _, r := range s.Rest {
		out = append(out, emitSimple(r))
	}
	return out
}

func emitSimple(s *parser.SimpleStmt) StmtNode {
	pos := pos(s.Pos)
	switch {
	case s.Return != nil:
		return emitReturn(s.Return)
	case s.Pass:
		return &Pass{Pos: pos}
	case s.Break:
		return &Break{Pos: pos}
	case s.Continue:
		return &Continue{Pos: pos}
	case s.Raise != nil:
		return emitRaise(s.Raise)
	case s.Del != nil:
		return emitDel(s.Del)
	case s.Global != nil:
		return &Global{Pos: pos, Names: s.Global.Names}
	case s.Nonlocal != nil:
		return &Nonlocal{Pos: pos, Names: s.Nonlocal.Names}
	case s.Assert != nil:
		a := s.Assert
		return &Assert{Pos: pos, Test: emitExpr(a.Test), Msg: emitExprOpt(a.Msg)}
	case s.Import != nil:
		return emitImport(s.Import)
	case s.From != nil:
		return emitFrom(s.From)
	case s.Yield != nil:
		return &Expr{Pos: pos, Value: emitYield(s.Yield.Expr)}
	case s.Assign != nil:
		return emitAssign(s.Assign)
	case s.ExprStmt != nil:
		return &Expr{Pos: pos, Value: emitExpr(s.ExprStmt)}
	}
	return nil
}

func emitReturn(r *parser.ReturnStmt) StmtNode {
	p := pos(r.Pos)
	switch len(r.Values) {
	case 0:
		return &Return{Pos: p}
	case 1:
		return &Return{Pos: p, Value: emitExpr(r.Values[0])}
	default:
		elts := make([]ExprNode, 0, len(r.Values))
		for _, v := range r.Values {
			elts = append(elts, emitExpr(v))
		}
		return &Return{Pos: p, Value: &Tuple{Pos: p, Elts: elts, Ctx: &Load{}}}
	}
}

func emitRaise(r *parser.RaiseStmt) StmtNode {
	return &Raise{Pos: pos(r.Pos), Exc: emitExprOpt(r.Exc), Cause: emitExprOpt(r.Cause)}
}

func emitDel(d *parser.DelStmt) StmtNode {
	targets := make([]ExprNode, 0, len(d.Targets))
	for _, t := range d.Targets {
		targets = append(targets, withCtx(emitExpr(t), &Del{}))
	}
	return &Delete{Pos: pos(d.Pos), Targets: targets}
}

func emitImport(im *parser.ImportStmt) StmtNode {
	names := make([]*Alias, 0, len(im.Names))
	for _, n := range im.Names {
		names = append(names, &Alias{
			Pos:    pos(n.Pos),
			Name:   strings.Join(n.Name.Parts, "."),
			Asname: n.Asname,
		})
	}
	return &Import{Pos: pos(im.Pos), Names: names}
}

func emitFrom(fr *parser.FromStmt) StmtNode {
	level := 0
	for _, d := range fr.Dots {
		if d == "..." {
			level += 3
		} else {
			level++
		}
	}
	module := ""
	if fr.Module != nil {
		module = strings.Join(fr.Module.Parts, ".")
	}
	var names []*Alias
	switch {
	case fr.Star:
		names = []*Alias{{Pos: pos(fr.Pos), Name: "*"}}
	case len(fr.Group) > 0:
		names = make([]*Alias, 0, len(fr.Group))
		for _, n := range fr.Group {
			names = append(names, &Alias{Pos: pos(n.Pos), Name: n.Name, Asname: n.Asname})
		}
	default:
		names = make([]*Alias, 0, len(fr.Plain))
		for _, n := range fr.Plain {
			names = append(names, &Alias{Pos: pos(n.Pos), Name: n.Name, Asname: n.Asname})
		}
	}
	return &ImportFrom{Pos: pos(fr.Pos), Module: module, Names: names, Level: level}
}

func emitAssign(a *parser.AssignStmt) StmtNode {
	p := pos(a.Pos)
	switch {
	case a.Annot != nil:
		// AnnAssign. Simple=1 only when the target is a bare Name.
		target := withCtx(emitExpr(a.Target), &Store{})
		simple := 0
		if _, ok := target.(*Name); ok {
			simple = 1
		}
		return &AnnAssign{
			Pos:        p,
			Target:     target,
			Annotation: emitExpr(a.Annot),
			Value:      emitExprOpt(a.AnnVal),
			Simple:     simple,
		}
	case a.Aug != "":
		return &AugAssign{
			Pos:    p,
			Target: withCtx(emitExpr(a.Target), &Store{}),
			Op:     augOp(a.Aug),
			Value:  emitExpr(a.AugVal),
		}
	case len(a.More) > 0:
		// Chained `a = b = c = expr`. The final expression is the rvalue;
		// every preceding capture (Target plus all but the last in More)
		// becomes a Store target.
		all := append([]*parser.Expression{a.Target}, a.More...)
		val := emitExpr(all[len(all)-1])
		targets := make([]ExprNode, 0, len(all)-1)
		for _, t := range all[:len(all)-1] {
			targets = append(targets, withCtx(emitExpr(t), &Store{}))
		}
		return &Assign{Pos: p, Targets: targets, Value: val}
	default:
		// Bare expression statement reuses Assign's target slot — but
		// SimpleStmt routes those through ExprStmt, not here. Reaching
		// this branch means the parser captured Target with no operator,
		// which only happens during partial parses; treat it as Expr.
		return &Expr{Pos: p, Value: emitExpr(a.Target)}
	}
}

func augOp(op string) OperatorNode {
	switch op {
	case "+=":
		return &Add{}
	case "-=":
		return &Sub{}
	case "*=":
		return &Mult{}
	case "/=":
		return &Div{}
	case "//=":
		return &FloorDiv{}
	case "%=":
		return &Mod{}
	case "@=":
		return &MatMult{}
	case "&=":
		return &BitAnd{}
	case "|=":
		return &BitOr{}
	case "^=":
		return &BitXor{}
	case "<<=":
		return &LShift{}
	case ">>=":
		return &RShift{}
	case "**=":
		return &Pow{}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Compound statements
// ---------------------------------------------------------------------------

func emitCompound(c *parser.CompoundStmt) StmtNode {
	switch {
	case c.Decorated != nil:
		return emitDecorated(c.Decorated)
	case c.If != nil:
		return emitIf(c.If)
	case c.While != nil:
		w := c.While
		return &While{
			Pos:    pos(w.Pos),
			Test:   emitExpr(w.Test),
			Body:   emitBlock(w.Body),
			Orelse: emitElse(w.Else),
		}
	case c.For != nil:
		f := c.For
		return &For{
			Pos:    pos(f.Pos),
			Target: emitTargetList(f.Target, &Store{}),
			Iter:   emitExpr(f.Iter),
			Body:   emitBlock(f.Body),
			Orelse: emitElse(f.Else),
		}
	case c.With != nil:
		w := c.With
		raw := w.Items
		if !w.Paren {
			raw = w.BareItems
		}
		items := make([]*Withitem, 0, len(raw))
		for _, it := range raw {
			items = append(items, &Withitem{
				ContextExpr:  emitExpr(it.Context),
				OptionalVars: ifNotNil(it.Vars, func(e *parser.Expression) ExprNode { return withCtx(emitExpr(e), &Store{}) }),
			})
		}
		return &With{Pos: pos(w.Pos), Items: items, Body: emitBlock(w.Body)}
	case c.Try != nil:
		return emitTry(c.Try)
	case c.FuncDef != nil:
		return emitFuncDef(c.FuncDef)
	case c.ClassDef != nil:
		return emitClassDef(c.ClassDef)
	}
	return nil
}

func emitIf(s *parser.IfStmt) StmtNode {
	root := &If{Pos: pos(s.Pos), Test: emitExpr(s.Test), Body: emitBlock(s.Body)}
	cur := root
	for _, e := range s.Elifs {
		next := &If{Pos: pos(e.Pos), Test: emitExpr(e.Test), Body: emitBlock(e.Body)}
		cur.Orelse = []StmtNode{next}
		cur = next
	}
	if s.Else != nil {
		cur.Orelse = emitBlock(s.Else.Body)
	}
	return root
}

func emitTry(t *parser.TryStmt) StmtNode {
	handlers := make([]ExcepthandlerNode, 0, len(t.Handlers))
	for _, h := range t.Handlers {
		handlers = append(handlers, &ExceptHandler{
			Pos:  pos(h.Pos),
			Type: emitExprOpt(h.Type),
			Name: h.Name,
			Body: emitBlock(h.Body),
		})
	}
	out := &Try{
		Pos:      pos(t.Pos),
		Body:     emitBlock(t.Body),
		Handlers: handlers,
		Orelse:   emitElse(t.Else),
	}
	if t.Finally != nil {
		out.Finalbody = emitBlock(t.Finally.Body)
	}
	return out
}

func emitFuncDef(f *parser.FuncDef) StmtNode {
	return &FunctionDef{
		Pos:     pos(f.Pos),
		Name:    f.Name,
		Args:    emitArguments(f.Params),
		Body:    emitBlock(f.Body),
		Returns: emitExprOpt(f.Returns),
	}
}

func emitClassDef(c *parser.ClassDef) StmtNode {
	bases, kws := emitArgList(c.Bases)
	return &ClassDef{
		Pos:      pos(c.Pos),
		Name:     c.Name,
		Bases:    bases,
		Keywords: kws,
		Body:     emitBlock(c.Body),
	}
}

// emitDecorated lifts a decorator stack onto the function or class
// definition that follows. CPython models decorators as a list field on
// the def/class node, in source order.
func emitDecorated(d *parser.Decorated) StmtNode {
	decos := make([]ExprNode, 0, len(d.Decorators))
	for _, dec := range d.Decorators {
		decos = append(decos, emitExpr(dec.Expr))
	}
	switch {
	case d.FuncDef != nil:
		fd := emitFuncDef(d.FuncDef).(*FunctionDef)
		fd.DecoratorList = decos
		return fd
	case d.ClassDef != nil:
		cd := emitClassDef(d.ClassDef).(*ClassDef)
		cd.DecoratorList = decos
		return cd
	}
	return nil
}

func emitBlock(b *parser.Block) []StmtNode {
	out := make([]StmtNode, 0, len(b.Body))
	for _, st := range b.Body {
		out = append(out, emitStatements(st)...)
	}
	return out
}

func emitElse(e *parser.ElseClause) []StmtNode {
	if e == nil {
		return nil
	}
	return emitBlock(e.Body)
}

// emitArguments turns the bootstrap Param list into the full Arguments
// product type. The bootstrap grammar doesn't track posonly/kwonly
// markers; everything lives in Args, *vararg, or **kwarg as recognised
// by Param.Star and Param.Double. Defaults follow CPython's convention:
// only the trailing run of positional defaults goes in Defaults.
// emitArguments turns the participle Param list into the Arguments product
// type. The list is walked in source order. A `/` flips already-collected
// positional args into Posonlyargs (PEP 570). A `*` (with or without name)
// flips subsequent regular params into Kwonlyargs (PEP 3102). `**name`
// becomes Kwarg.
func emitArguments(params []*parser.Param) *Arguments {
	args := &Arguments{}
	seenStar := false
	for _, p := range params {
		switch {
		case p.Slash:
			args.Posonlyargs = append(args.Posonlyargs, args.Args...)
			args.Args = nil
		case p.Star:
			seenStar = true
			if p.Name != "" {
				args.Vararg = paramArg(p)
			}
		case p.Double:
			args.Kwarg = paramArg(p)
		case seenStar:
			args.Kwonlyargs = append(args.Kwonlyargs, paramArg(p))
			if p.Default != nil {
				args.KwDefaults = append(args.KwDefaults, emitExpr(p.Default))
			} else {
				args.KwDefaults = append(args.KwDefaults, nil)
			}
		default:
			args.Args = append(args.Args, paramArg(p))
			if p.Default != nil {
				args.Defaults = append(args.Defaults, emitExpr(p.Default))
			}
		}
	}
	return args
}

func paramArg(p *parser.Param) *Arg {
	return &Arg{Pos: pos(p.Pos), Arg: p.Name, Annotation: emitExprOpt(p.Annot)}
}

// emitArgList splits a participle Argument list into positional bases and
// keyword bases for ClassDef. Star (*x) and DStar (**x) variants become
// positional Starred and unnamed Keyword respectively, which is what
// CPython does for class definitions.
func emitArgList(args []*parser.Argument) ([]ExprNode, []*Keyword) {
	var bases []ExprNode
	var kws []*Keyword
	for _, a := range args {
		switch {
		case a.DStar != nil:
			kws = append(kws, &Keyword{Pos: pos(a.Pos), Value: emitExpr(a.DStar)})
		case a.Star != nil:
			bases = append(bases, &Starred{Pos: pos(a.Pos), Value: emitExpr(a.Star), Ctx: &Load{}})
		case a.Kwarg != nil:
			kws = append(kws, &Keyword{Pos: pos(a.Kwarg.Pos), Arg: a.Kwarg.Name, Value: emitExpr(a.Kwarg.Value)})
		case a.Posn != nil:
			bases = append(bases, emitExpr(a.Posn))
		}
	}
	return bases, kws
}

// emitTargetList turns a TargetList into either a single expression (when
// there's no comma) or a Tuple (otherwise). The supplied context is
// applied recursively so each Name/Attribute/Subscript inside a tuple
// target gets the right Ctx.
func emitTargetList(t *parser.TargetList, ctx ExprContextNode) ExprNode {
	if len(t.Tail) == 0 && !t.HasTrail {
		return withCtx(emitTargetAtom(t.Head), ctx)
	}
	elts := make([]ExprNode, 0, 1+len(t.Tail))
	elts = append(elts, withCtx(emitTargetAtom(t.Head), ctx))
	for _, a := range t.Tail {
		elts = append(elts, withCtx(emitTargetAtom(a), ctx))
	}
	return &Tuple{Pos: pos(t.Pos), Elts: elts, Ctx: ctx}
}

func emitTargetAtom(a *parser.TargetAtom) ExprNode {
	e := emitBitOr(a.Expr)
	if a.Star {
		return &Starred{Pos: pos(a.Pos), Value: e, Ctx: &Load{}}
	}
	return e
}

// ---------------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------------

func emitExprOpt(e *parser.Expression) ExprNode {
	if e == nil {
		return nil
	}
	return emitExpr(e)
}

func emitExpr(e *parser.Expression) ExprNode {
	if e.Walrus != nil {
		w := e.Walrus
		return &NamedExpr{
			Pos:    pos(w.Pos),
			Target: &Name{Pos: pos(w.Pos), Id: w.Name, Ctx: &Store{}},
			Value:  emitExpr(w.Value),
		}
	}
	if e.Lambda != nil {
		l := e.Lambda
		return &Lambda{Pos: pos(l.Pos), Args: emitArguments(l.Params), Body: emitExpr(l.Body)}
	}
	body := emitDisjunction(e.Body)
	if e.IfTest != nil {
		return &IfExp{
			Pos:    pos(e.Pos),
			Test:   emitDisjunction(e.IfTest),
			Body:   body,
			Orelse: emitExpr(e.IfElse),
		}
	}
	return body
}

func emitDisjunction(d *parser.Disjunction) ExprNode {
	if len(d.Tail) == 0 {
		return emitConjunction(d.Head)
	}
	values := make([]ExprNode, 0, 1+len(d.Tail))
	values = append(values, emitConjunction(d.Head))
	for _, c := range d.Tail {
		values = append(values, emitConjunction(c))
	}
	return &BoolOp{Pos: pos(d.Pos), Op: &Or{}, Values: values}
}

func emitConjunction(c *parser.Conjunction) ExprNode {
	if len(c.Tail) == 0 {
		return emitInversion(c.Head)
	}
	values := make([]ExprNode, 0, 1+len(c.Tail))
	values = append(values, emitInversion(c.Head))
	for _, i := range c.Tail {
		values = append(values, emitInversion(i))
	}
	return &BoolOp{Pos: pos(c.Pos), Op: &And{}, Values: values}
}

func emitInversion(i *parser.Inversion) ExprNode {
	if i.Not {
		return &UnaryOp{Pos: pos(i.Pos), Op: &Not{}, Operand: emitInversion(i.Inv)}
	}
	return emitComparison(i.Comp)
}

func emitComparison(c *parser.Comparison) ExprNode {
	if len(c.Tail) == 0 {
		return emitBitOr(c.Head)
	}
	ops := make([]CmpopNode, 0, len(c.Tail))
	rhs := make([]ExprNode, 0, len(c.Tail))
	for _, r := range c.Tail {
		ops = append(ops, cmpOp(r))
		rhs = append(rhs, emitBitOr(r.RHS))
	}
	return &Compare{Pos: pos(c.Pos), Left: emitBitOr(c.Head), Ops: ops, Comparators: rhs}
}

func cmpOp(r *parser.CmpRight) CmpopNode {
	switch {
	case r.NotIn:
		return &NotIn{}
	case r.Is:
		if r.IsNot {
			return &IsNot{}
		}
		return &Is{}
	}
	switch r.Op {
	case "==":
		return &Eq{}
	case "!=":
		return &NotEq{}
	case "<":
		return &Lt{}
	case "<=":
		return &LtE{}
	case ">":
		return &Gt{}
	case ">=":
		return &GtE{}
	case "in":
		return &In{}
	}
	return nil
}

func emitBitOr(b *parser.BitOr) ExprNode {
	if len(b.Tail) == 0 {
		return emitBitXor(b.Head)
	}
	cur := emitBitXor(b.Head)
	for _, x := range b.Tail {
		cur = &BinOp{Pos: pos(b.Pos), Left: cur, Op: &BitOr{}, Right: emitBitXor(x)}
	}
	return cur
}

func emitBitXor(b *parser.BitXor) ExprNode {
	if len(b.Tail) == 0 {
		return emitBitAnd(b.Head)
	}
	cur := emitBitAnd(b.Head)
	for _, x := range b.Tail {
		cur = &BinOp{Pos: pos(b.Pos), Left: cur, Op: &BitXor{}, Right: emitBitAnd(x)}
	}
	return cur
}

func emitBitAnd(b *parser.BitAnd) ExprNode {
	if len(b.Tail) == 0 {
		return emitShift(b.Head)
	}
	cur := emitShift(b.Head)
	for _, x := range b.Tail {
		cur = &BinOp{Pos: pos(b.Pos), Left: cur, Op: &BitAnd{}, Right: emitShift(x)}
	}
	return cur
}

func emitShift(s *parser.Shift) ExprNode {
	if len(s.Tail) == 0 {
		return emitSum(s.Head)
	}
	cur := emitSum(s.Head)
	for _, x := range s.Tail {
		var op OperatorNode = &LShift{}
		if x.Op == ">>" {
			op = &RShift{}
		}
		cur = &BinOp{Pos: pos(s.Pos), Left: cur, Op: op, Right: emitSum(x.RHS)}
	}
	return cur
}

func emitSum(s *parser.Sum) ExprNode {
	if len(s.Tail) == 0 {
		return emitTerm(s.Head)
	}
	cur := emitTerm(s.Head)
	for _, x := range s.Tail {
		var op OperatorNode = &Add{}
		if x.Op == "-" {
			op = &Sub{}
		}
		cur = &BinOp{Pos: pos(s.Pos), Left: cur, Op: op, Right: emitTerm(x.RHS)}
	}
	return cur
}

func emitTerm(t *parser.Term) ExprNode {
	if len(t.Tail) == 0 {
		return emitFactor(t.Head)
	}
	cur := emitFactor(t.Head)
	for _, x := range t.Tail {
		op := termOp(x.Op)
		cur = &BinOp{Pos: pos(t.Pos), Left: cur, Op: op, Right: emitFactor(x.RHS)}
	}
	return cur
}

func termOp(s string) OperatorNode {
	switch s {
	case "*":
		return &Mult{}
	case "/":
		return &Div{}
	case "//":
		return &FloorDiv{}
	case "%":
		return &Mod{}
	case "@":
		return &MatMult{}
	}
	return nil
}

func emitFactor(f *parser.Factor) ExprNode {
	if f.Unary != "" {
		var op UnaryopNode
		switch f.Unary {
		case "+":
			op = &UAdd{}
		case "-":
			op = &USub{}
		case "~":
			op = &Invert{}
		}
		return &UnaryOp{Pos: pos(f.Pos), Op: op, Operand: emitFactor(f.Inner)}
	}
	return emitPower(f.Power)
}

// Power is right-associative: `a ** b ** c` is `a ** (b ** c)`.
func emitPower(p *parser.Power) ExprNode {
	base := emitAwaitPrimary(p.Await)
	if p.Exp == nil {
		return base
	}
	return &BinOp{Pos: pos(p.Pos), Left: base, Op: &Pow{}, Right: emitFactor(p.Exp)}
}

func emitAwaitPrimary(a *parser.AwaitPrimary) ExprNode {
	val := emitPrimary(a.Primary)
	if a.Await {
		return &Await{Pos: pos(a.Pos), Value: val}
	}
	return val
}

func emitPrimary(p *parser.Primary) ExprNode {
	cur := emitAtom(p.Atom)
	for _, t := range p.Trailers {
		cur = applyTrailer(cur, t)
	}
	return cur
}

func applyTrailer(value ExprNode, t *parser.Trailer) ExprNode {
	switch {
	case t.Attr != "":
		return &Attribute{Pos: pos(t.Pos), Value: value, Attr: t.Attr, Ctx: &Load{}}
	case t.Call != nil:
		args, kws := emitArgList(t.Call.Args)
		return &Call{Pos: pos(t.Pos), Func: value, Args: args, Keywords: kws}
	case t.Sub != nil:
		return &Subscript{Pos: pos(t.Pos), Value: value, Slice: emitSubscriptList(t.Sub), Ctx: &Load{}}
	}
	return value
}

func emitSubscriptList(sl *parser.SubscriptList) ExprNode {
	if len(sl.Items) == 1 {
		return emitSubscript(sl.Items[0])
	}
	elts := make([]ExprNode, 0, len(sl.Items))
	for _, s := range sl.Items {
		elts = append(elts, emitSubscript(s))
	}
	return &Tuple{Pos: pos(sl.Pos), Elts: elts, Ctx: &Load{}}
}

func emitSubscript(s *parser.Subscript) ExprNode {
	if s.Slice == nil {
		return emitExpr(s.Lower)
	}
	out := &Slice{Pos: pos(s.Pos)}
	if s.Lower != nil {
		out.Lower = emitExpr(s.Lower)
	}
	if s.Slice.Upper != nil {
		out.Upper = emitExpr(s.Slice.Upper)
	}
	if s.Slice.Step != nil && s.Slice.Step.Value != nil {
		out.Step = emitExpr(s.Slice.Step.Value)
	}
	return out
}

func emitAtom(a *parser.Atom) ExprNode {
	p := pos(a.Pos)
	switch {
	case a.Name != "":
		return &Name{Pos: p, Id: a.Name, Ctx: &Load{}}
	case a.Number != "":
		return &Constant{Pos: p, Value: numberConstant(a.Number)}
	case len(a.String) > 0:
		return stringConstant(p, a.String)
	case a.True:
		return &Constant{Pos: p, Value: ConstantValue{Kind: ConstantBool, Bool: true}}
	case a.False_:
		return &Constant{Pos: p, Value: ConstantValue{Kind: ConstantBool, Bool: false}}
	case a.None:
		return &Constant{Pos: p, Value: ConstantValue{Kind: ConstantNone}}
	case a.Ellipsis:
		return &Constant{Pos: p, Value: ConstantValue{Kind: ConstantEllipsis}}
	case a.List != nil:
		elts := make([]ExprNode, 0, len(a.List.Elts))
		for _, e := range a.List.Elts {
			elts = append(elts, emitStarOrExpr(e))
		}
		return &List{Pos: p, Elts: elts, Ctx: &Load{}}
	case a.Dict != nil:
		return emitDictOrSet(p, a.Dict)
	case a.Paren != nil:
		return emitParen(p, a.Paren)
	}
	return nil
}

// emitDictOrSet picks Dict vs Set based on the first item: a `**x` or a
// `key: value` pair makes it a Dict; anything else is a Set. Dict
// unpacking renders as a key=None entry in CPython's AST, so we set the
// nil key explicitly. Set literals accept `*x` (star-unpack) but reject
// `**x`; mixing them in source is rejected by CPython, not us.
func emitDictOrSet(p Pos, d *parser.DictOrSetLit) ExprNode {
	if d.First == nil {
		return &Dict{Pos: p}
	}
	isDict := d.First.DStar != nil || d.First.Value != nil
	if !isDict {
		elts := []ExprNode{emitDictSetElt(d.First)}
		for _, r := range d.Rest {
			elts = append(elts, emitDictSetElt(r))
		}
		return &Set{Pos: p, Elts: elts}
	}
	var keys, values []ExprNode
	addItem := func(it *parser.DictItemOrExpr) {
		if it.DStar != nil {
			keys = append(keys, nil)
			values = append(values, emitExpr(it.DStar))
			return
		}
		keys = append(keys, emitExpr(it.Key))
		values = append(values, emitExpr(it.Value))
	}
	addItem(d.First)
	for _, r := range d.Rest {
		addItem(r)
	}
	return &Dict{Pos: p, Keys: keys, Values: values}
}

func emitDictSetElt(it *parser.DictItemOrExpr) ExprNode {
	if it.StarSet != nil {
		return &Starred{Pos: pos(it.Pos), Value: emitExpr(it.StarSet), Ctx: &Load{}}
	}
	return emitExpr(it.Key)
}

// emitParen distinguishes `(expr)` (just a value) from `(a,)` /
// `(a, b)` (Tuple). A single element with no trailing comma is the
// parenthesized expression itself; CPython keeps this distinction.
//
// Detecting the trailing comma after the parser has consumed it is
// awkward: ParenLit's Elts field is the same shape whether or not a
// trailing comma was present. The bootstrap grammar therefore can't
// represent a single-element tuple `(x,)` separately from `(x)` — we
// always fold a single element to its bare expression. PR2 widens
// ParenLit so the trailing-comma case becomes detectable.
func emitParen(p Pos, paren *parser.ParenLit) ExprNode {
	switch len(paren.Elts) {
	case 0:
		return &Tuple{Pos: p, Ctx: &Load{}}
	case 1:
		// `(*x,)` is a Tuple even with a single element because the comma
		// is implicit; same idea as `(x,)`. A bare `(*x)` is invalid in
		// CPython but we mirror the parser shape and let downstream flag.
		single := paren.Elts[0]
		if paren.TrailingComma || single.Star != nil {
			return &Tuple{Pos: p, Elts: []ExprNode{emitStarOrExpr(single)}, Ctx: &Load{}}
		}
		return emitExpr(single.Expr)
	default:
		elts := make([]ExprNode, 0, len(paren.Elts))
		for _, e := range paren.Elts {
			elts = append(elts, emitStarOrExpr(e))
		}
		return &Tuple{Pos: p, Elts: elts, Ctx: &Load{}}
	}
}

// emitStarOrExpr unwraps a list/tuple element that may be `*expr`.
func emitStarOrExpr(s *parser.StarOrExpr) ExprNode {
	if s.Star != nil {
		return &Starred{Pos: pos(s.Pos), Value: emitExpr(s.Star), Ctx: &Load{}}
	}
	return emitExpr(s.Expr)
}

func emitYield(y *parser.YieldExpr) ExprNode {
	p := pos(y.Pos)
	if y.From != nil {
		return &YieldFrom{Pos: p, Value: emitExpr(y.From)}
	}
	if y.Val != nil {
		return &Yield{Pos: p, Value: emitExpr(y.Val)}
	}
	return &Yield{Pos: p}
}

// numberConstant turns a NUMBER token's literal text into a ConstantValue.
// Integer literals stay as their original text (Python keeps unbounded
// integers). Floats and complex parse via strconv. The parser strips PEP
// 515 underscores already, but we strip a trailing 'j'/'J' for complex.
func numberConstant(text string) ConstantValue {
	t := strings.ReplaceAll(text, "_", "")
	if strings.HasSuffix(t, "j") || strings.HasSuffix(t, "J") {
		f, _ := strconv.ParseFloat(t[:len(t)-1], 64)
		return ConstantValue{Kind: ConstantComplex, Imag: f}
	}
	if strings.ContainsAny(t, ".eE") && !isHexInt(t) {
		f, _ := strconv.ParseFloat(t, 64)
		return ConstantValue{Kind: ConstantFloat, Float: f}
	}
	return ConstantValue{Kind: ConstantInt, Int: t}
}

func isHexInt(s string) bool {
	return len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X')
}

// stringConstant concatenates adjacent string literals (Python's implicit
// concatenation: `"a" "b"` is `"ab"`) and decodes their content. The
// bootstrap lexer keeps the surrounding quotes and prefixes; we strip
// them naively here. F-strings and t-strings get deferred to PR2 — they
// still flow through here as literal STRING tokens.
func stringConstant(p Pos, parts []string) ExprNode {
	isBytes := false
	for _, raw := range parts {
		if hasStringPrefix(raw, 'b') {
			isBytes = true
			break
		}
	}
	var b strings.Builder
	for _, raw := range parts {
		b.WriteString(decodeStringLiteral(raw))
	}
	if isBytes {
		return &Constant{Pos: p, Value: ConstantValue{Kind: ConstantBytes, Bytes: []byte(b.String())}}
	}
	return &Constant{Pos: p, Value: ConstantValue{Kind: ConstantStr, Str: b.String()}}
}

// hasStringPrefix reports whether the raw STRING token carries the given
// prefix letter (case-insensitive) before its opening quote.
func hasStringPrefix(raw string, want byte) bool {
	want |= 0x20
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '\'' || c == '"' {
			return false
		}
		if c|0x20 == want {
			return true
		}
	}
	return false
}

// decodeStringLiteral strips the leading prefix and matching quote pair
// from a raw STRING token, returning the inner text. Escape sequence
// handling is intentionally minimal in the bootstrap; Python accepts any
// of `\n`, `\t`, `\\`, `\'`, `\"` as escapes inside a regular string,
// but the rich set (octal, hex, unicode names, ...) lands in PR2 along
// with f/t-string interpolation.
func decodeStringLiteral(raw string) string {
	s := raw
	for len(s) > 0 {
		c := s[0]
		if c == '\'' || c == '"' {
			break
		}
		s = s[1:]
	}
	if len(s) >= 6 && (strings.HasPrefix(s, `"""`) || strings.HasPrefix(s, `'''`)) {
		quote := s[:3]
		body := strings.TrimPrefix(s, quote)
		body = strings.TrimSuffix(body, quote)
		return decodeEscapes(body)
	}
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		body := s[1 : len(s)-1]
		return decodeEscapes(body)
	}
	return s
}

// decodeEscapes processes Python's escape sequences inside a string literal:
//   \\ \' \" \a \b \f \n \r \t \v   single-char escapes
//   \NNN                             1-3 octal digits, value mod 256
//   \xHH                             two hex digits, exact
//   \uHHHH                           four hex digits (BMP code point)
//   \UHHHHHHHH                       eight hex digits (full code point)
//   \<newline>                       line continuation, dropped
// Anything else (e.g. \z, \q) is preserved as-is, matching CPython's
// "deprecated invalid escape" behavior; \N{name} is not yet implemented
// and is left intact rather than failing parse.
func decodeEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		c := s[i+1]
		switch c {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		case '\'':
			b.WriteByte('\'')
		case '"':
			b.WriteByte('"')
		case 'a':
			b.WriteByte(0x07)
		case 'b':
			b.WriteByte(0x08)
		case 'f':
			b.WriteByte(0x0c)
		case 'v':
			b.WriteByte(0x0b)
		case '\n':
			// Backslash-newline is a line continuation: drop both.
		case 'x':
			if i+3 < len(s) && isHex(s[i+2]) && isHex(s[i+3]) {
				b.WriteByte(hexNibble(s[i+2])<<4 | hexNibble(s[i+3]))
				i += 3
				continue
			}
			b.WriteByte('\\')
			b.WriteByte(c)
		case '0', '1', '2', '3', '4', '5', '6', '7':
			// Octal: 1-3 digits, value mod 256.
			val := int(c - '0')
			n := 1
			if i+2 < len(s) && isOctal(s[i+2]) {
				val = val*8 + int(s[i+2]-'0')
				n++
				if i+3 < len(s) && isOctal(s[i+3]) {
					val = val*8 + int(s[i+3]-'0')
					n++
				}
			}
			b.WriteByte(byte(val & 0xff))
			i += n
			continue
		case 'u':
			if i+5 < len(s) && allHex(s[i+2:i+6]) {
				r := hexValue(s[i+2 : i+6])
				b.WriteRune(rune(r))
				i += 5
				continue
			}
			b.WriteByte('\\')
			b.WriteByte(c)
		case 'U':
			if i+9 < len(s) && allHex(s[i+2:i+10]) {
				r := hexValue(s[i+2 : i+10])
				b.WriteRune(rune(r))
				i += 9
				continue
			}
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
		i++
	}
	return b.String()
}

func isOctal(c byte) bool { return c >= '0' && c <= '7' }

func allHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isHex(s[i]) {
			return false
		}
	}
	return true
}

func hexValue(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n<<4 | int(hexNibble(s[i]))
	}
	return n
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c|0x20 >= 'a' && c|0x20 <= 'f')
}

func hexNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	default:
		return c|0x20 - 'a' + 10
	}
}

// withCtx replaces the Ctx field of Name/Attribute/Subscript/Starred/
// Tuple/List nodes. Any other expression type passes through unchanged
// (CPython rejects them as targets at compile time, not parse time).
func withCtx(e ExprNode, ctx ExprContextNode) ExprNode {
	switch n := e.(type) {
	case *Name:
		n.Ctx = ctx
	case *Attribute:
		n.Ctx = ctx
	case *Subscript:
		n.Ctx = ctx
	case *Starred:
		n.Ctx = ctx
		n.Value = withCtx(n.Value, ctx)
	case *Tuple:
		n.Ctx = ctx
		for i, el := range n.Elts {
			n.Elts[i] = withCtx(el, ctx)
		}
	case *List:
		n.Ctx = ctx
		for i, el := range n.Elts {
			n.Elts[i] = withCtx(el, ctx)
		}
	}
	return e
}

func ifNotNil[T any](v *T, f func(*T) ExprNode) ExprNode {
	if v == nil {
		return nil
	}
	return f(v)
}

// pos copies a participle Position into the AST shape. The lexer adapter
// already turned Col into 1-indexed in printable output; CPython AST keeps
// col_offset as 0-indexed UTF-8 bytes, which matches what lex emits, so
// we subtract one here to get back to 0-indexed.
func pos(p plexer.Position) Pos {
	return Pos{Lineno: p.Line, ColOffset: p.Column - 1, EndLineno: p.Line, EndColOffset: p.Column - 1}
}
