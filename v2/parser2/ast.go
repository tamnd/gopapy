package parser2

import (
	"fmt"
	"strings"
)

// Pos is the 1-indexed source position attached to every node.
type Pos struct {
	Line int
	Col  int
}

// Expr is the closed sum type for parser2 expression nodes. Concrete
// types mirror v1's ast.ExprNode shapes so a future converter is
// mechanical. v2 keeps its own copies because Go's strict
// major-version-suffix rule prevents v2 from importing v1 directly
// while v1 ships under the /v1 suffix with v0.x.x tags.
type Expr interface {
	exprNode()
	pos() Pos
}

// Constant is an int, float, complex, string, bytes, None, True,
// False, or Ellipsis literal. Value is the Go-typed payload; Kind
// names the source form for the dump representation.
type Constant struct {
	P     Pos
	Kind  string
	Value any
}

func (*Constant) exprNode()  {}
func (c *Constant) pos() Pos { return c.P }

// Name is a bare identifier reference.
type Name struct {
	P  Pos
	Id string
}

func (*Name) exprNode()  {}
func (n *Name) pos() Pos { return n.P }

// UnaryOp is a prefix operator application.
type UnaryOp struct {
	P       Pos
	Op      string
	Operand Expr
}

func (*UnaryOp) exprNode()  {}
func (u *UnaryOp) pos() Pos { return u.P }

// BinOp is a binary arithmetic / bitwise / shift operator
// application.
type BinOp struct {
	P     Pos
	Op    string
	Left  Expr
	Right Expr
}

func (*BinOp) exprNode()  {}
func (b *BinOp) pos() Pos { return b.P }

// BoolOp is an `and` or `or` chain. Python flattens chains into a
// single node with N>=2 operands.
type BoolOp struct {
	P      Pos
	Op     string
	Values []Expr
}

func (*BoolOp) exprNode()  {}
func (n *BoolOp) pos() Pos { return n.P }

// Compare is a chained comparison. ops and comparators are
// parallel slices; len(ops) == len(comparators) and Left is the
// first operand, comparators are the rest.
type Compare struct {
	P           Pos
	Left        Expr
	Ops         []string
	Comparators []Expr
}

func (*Compare) exprNode()  {}
func (n *Compare) pos() Pos { return n.P }

// Attribute is `value.attr`.
type Attribute struct {
	P     Pos
	Value Expr
	Attr  string
}

func (*Attribute) exprNode()  {}
func (n *Attribute) pos() Pos { return n.P }

// Subscript is `value[slice]`.
type Subscript struct {
	P     Pos
	Value Expr
	Slice Expr
}

func (*Subscript) exprNode()  {}
func (n *Subscript) pos() Pos { return n.P }

// Slice is the inner shape of `a[lo:hi:step]`.
type Slice struct {
	P     Pos
	Lower Expr
	Upper Expr
	Step  Expr
}

func (*Slice) exprNode()  {}
func (n *Slice) pos() Pos { return n.P }

// Call is `func(args, **kwargs)`.
type Call struct {
	P        Pos
	Func     Expr
	Args     []Expr
	Keywords []*Keyword
}

func (*Call) exprNode()  {}
func (n *Call) pos() Pos { return n.P }

// Keyword is `name=value` inside a Call's argument list. A nil Arg
// represents `**kwargs` (the value is the dict expr).
type Keyword struct {
	P     Pos
	Arg   string
	Value Expr
}

// List, Tuple, Dict, Set are collection literals.
type List struct {
	P    Pos
	Elts []Expr
}

func (*List) exprNode()  {}
func (n *List) pos() Pos { return n.P }

type Tuple struct {
	P    Pos
	Elts []Expr
}

func (*Tuple) exprNode()  {}
func (n *Tuple) pos() Pos { return n.P }

type Set struct {
	P    Pos
	Elts []Expr
}

func (*Set) exprNode()  {}
func (n *Set) pos() Pos { return n.P }

type Dict struct {
	P      Pos
	Keys   []Expr // nil entry == `**other`
	Values []Expr
}

func (*Dict) exprNode()  {}
func (n *Dict) pos() Pos { return n.P }

// Comprehension is one `for x in xs if p1 if p2 ...` clause within
// a comp/genexp. IsAsync flags `async for`.
type Comprehension struct {
	Target  Expr
	Iter    Expr
	Ifs     []Expr
	IsAsync bool
}

// ListComp, SetComp, DictComp, GeneratorExp are the four
// comprehension forms.
type ListComp struct {
	P     Pos
	Elt   Expr
	Gens  []*Comprehension
}

func (*ListComp) exprNode()  {}
func (n *ListComp) pos() Pos { return n.P }

type SetComp struct {
	P    Pos
	Elt  Expr
	Gens []*Comprehension
}

func (*SetComp) exprNode()  {}
func (n *SetComp) pos() Pos { return n.P }

type DictComp struct {
	P     Pos
	Key   Expr
	Value Expr
	Gens  []*Comprehension
}

func (*DictComp) exprNode()  {}
func (n *DictComp) pos() Pos { return n.P }

type GeneratorExp struct {
	P    Pos
	Elt  Expr
	Gens []*Comprehension
}

func (*GeneratorExp) exprNode()  {}
func (n *GeneratorExp) pos() Pos { return n.P }

// Lambda is `lambda args: body`. Args follows the same shape as a
// def's parameter list.
type Lambda struct {
	P    Pos
	Args *Arguments
	Body Expr
}

func (*Lambda) exprNode()  {}
func (n *Lambda) pos() Pos { return n.P }

// Arguments is the parameter list shared by Lambda (and, in
// v0.1.30+, FunctionDef). Order: posonly, args, vararg, kwonly,
// kwarg. Defaults map to args from the right (Python semantics).
type Arguments struct {
	PosOnly  []*Arg
	Args     []*Arg
	Vararg   *Arg // *args, nil if absent
	KwOnly   []*Arg
	KwOnlyDef []Expr // 1:1 with KwOnly; nil entry = no default
	Kwarg    *Arg   // **kwargs, nil if absent
	Defaults []Expr // for the tail of Args (rightmost N args)
}

// Arg is one parameter. Annotation may be nil.
type Arg struct {
	P          Pos
	Name       string
	Annotation Expr
}

// NamedExpr is the walrus `(x := expr)`.
type NamedExpr struct {
	P      Pos
	Target Expr
	Value  Expr
}

func (*NamedExpr) exprNode()  {}
func (n *NamedExpr) pos() Pos { return n.P }

// Starred is `*expr` in unpacking positions (call args, list/tuple
// literals, assignment targets in v0.1.30+).
type Starred struct {
	P     Pos
	Value Expr
}

func (*Starred) exprNode()  {}
func (n *Starred) pos() Pos { return n.P }

// Await is the `await expr` expression. Legal only inside an async
// function body; the parser doesn't enforce that — symbols.Build is
// where the context check belongs.
type Await struct {
	P     Pos
	Value Expr
}

func (*Await) exprNode() {}
func (n *Await) pos() Pos { return n.P }

// Yield is `yield expr` or bare `yield`. Value may be nil.
type Yield struct {
	P     Pos
	Value Expr
}

func (*Yield) exprNode() {}
func (n *Yield) pos() Pos { return n.P }

// YieldFrom is `yield from expr`. Always has a value.
type YieldFrom struct {
	P     Pos
	Value Expr
}

func (*YieldFrom) exprNode() {}
func (n *YieldFrom) pos() Pos { return n.P }

// JoinedStr is an f-string. Values is a list of plain string
// Constants and FormattedValue interpolations interleaved in source
// order.
type JoinedStr struct {
	P      Pos
	Values []Expr
}

func (*JoinedStr) exprNode()  {}
func (n *JoinedStr) pos() Pos { return n.P }

// FormattedValue is one `{...}` interpolation inside a JoinedStr.
// Conversion is -1 (none), 114 ('r'), 115 ('s'), or 97 ('a') —
// matching CPython's ast.dump integer codes. FormatSpec is nil or
// another JoinedStr.
type FormattedValue struct {
	P          Pos
	Value      Expr
	Conversion int
	FormatSpec Expr
}

func (*FormattedValue) exprNode()  {}
func (n *FormattedValue) pos() Pos { return n.P }

// TemplateStr is a PEP 750 t-string literal. Strings and
// Interpolations interleave: Strings has length N+1 where N is
// len(Interpolations).
type TemplateStr struct {
	P              Pos
	Strings        []*Constant
	Interpolations []*Interpolation
}

func (*TemplateStr) exprNode()  {}
func (n *TemplateStr) pos() Pos { return n.P }

// Interpolation is one `{...}` inside a TemplateStr. Str preserves
// the source text of the expression because PEP 750's Template
// keeps the original for repr.
type Interpolation struct {
	P          Pos
	Value      Expr
	Str        string
	Conversion int
	FormatSpec Expr
}

func (*Interpolation) exprNode()  {}
func (n *Interpolation) pos() Pos { return n.P }

// IfExp is the conditional expression `body if test else orelse`.
type IfExp struct {
	P      Pos
	Test   Expr
	Body   Expr
	OrElse Expr
}

func (*IfExp) exprNode()  {}
func (n *IfExp) pos() Pos { return n.P }

// Module is the top-level node returned by ParseFile / ParseString.
// Body is the sequence of top-level statements.
type Module struct {
	Body []Stmt
}

// Stmt is the closed sum type for parser2 statement nodes.
type Stmt interface {
	stmtNode()
	pos() Pos
}

// ExprStmt wraps a bare expression used as a statement (e.g. a
// function call, an attribute fetch, or a docstring literal).
type ExprStmt struct {
	P     Pos
	Value Expr
}

func (*ExprStmt) stmtNode() {}
func (s *ExprStmt) pos() Pos { return s.P }

// Assign is `t1 = t2 = ... = value`. Targets has length >= 1.
type Assign struct {
	P       Pos
	Targets []Expr
	Value   Expr
}

func (*Assign) stmtNode() {}
func (s *Assign) pos() Pos { return s.P }

// AugAssign is `target op= value` (e.g. `x += 1`).
type AugAssign struct {
	P      Pos
	Target Expr
	Op     string
	Value  Expr
}

func (*AugAssign) stmtNode() {}
func (s *AugAssign) pos() Pos { return s.P }

// AnnAssign is `target: ann = value` or `target: ann`. Simple is true
// when target is a bare Name (not parenthesised, attribute, or
// subscript).
type AnnAssign struct {
	P          Pos
	Target     Expr
	Annotation Expr
	Value      Expr // nil when no = clause
	Simple     bool
}

func (*AnnAssign) stmtNode() {}
func (s *AnnAssign) pos() Pos { return s.P }

// Return is `return value` or bare `return`. Value may be nil.
type Return struct {
	P     Pos
	Value Expr
}

func (*Return) stmtNode() {}
func (s *Return) pos() Pos { return s.P }

// Raise is `raise exc from cause`. Both fields may be nil for a bare
// `raise`.
type Raise struct {
	P     Pos
	Exc   Expr
	Cause Expr
}

func (*Raise) stmtNode() {}
func (s *Raise) pos() Pos { return s.P }

// Pass / Break / Continue are the three keyword-only simple
// statements.
type Pass struct{ P Pos }

func (*Pass) stmtNode() {}
func (s *Pass) pos() Pos { return s.P }

type Break struct{ P Pos }

func (*Break) stmtNode() {}
func (s *Break) pos() Pos { return s.P }

type Continue struct{ P Pos }

func (*Continue) stmtNode() {}
func (s *Continue) pos() Pos { return s.P }

// Alias is `name as asname` inside Import / ImportFrom. Asname empty
// means no rename.
type Alias struct {
	P      Pos
	Name   string
	Asname string
}

// Import is `import a, b as c, d.e`.
type Import struct {
	P     Pos
	Names []*Alias
}

func (*Import) stmtNode() {}
func (s *Import) pos() Pos { return s.P }

// ImportFrom is `from .module import x, y as z`. Level counts the
// leading dots (0 for absolute).
type ImportFrom struct {
	P      Pos
	Module string // empty when only dots
	Names  []*Alias
	Level  int
}

func (*ImportFrom) stmtNode() {}
func (s *ImportFrom) pos() Pos { return s.P }

// Global / Nonlocal name declarations.
type Global struct {
	P     Pos
	Names []string
}

func (*Global) stmtNode() {}
func (s *Global) pos() Pos { return s.P }

type Nonlocal struct {
	P     Pos
	Names []string
}

func (*Nonlocal) stmtNode() {}
func (s *Nonlocal) pos() Pos { return s.P }

// Delete is `del a, b.c, d[0]`.
type Delete struct {
	P       Pos
	Targets []Expr
}

func (*Delete) stmtNode() {}
func (s *Delete) pos() Pos { return s.P }

// Assert is `assert test, msg`. Msg may be nil.
type Assert struct {
	P    Pos
	Test Expr
	Msg  Expr
}

func (*Assert) stmtNode() {}
func (s *Assert) pos() Pos { return s.P }

// If is `if test: body` plus optional `elif`/`else` chain. Elif lowers
// to a nested If inside Orelse.
type If struct {
	P      Pos
	Test   Expr
	Body   []Stmt
	Orelse []Stmt
}

func (*If) stmtNode() {}
func (s *If) pos() Pos { return s.P }

// While is `while test: body` with optional else. Orelse runs when the
// loop exits without break.
type While struct {
	P      Pos
	Test   Expr
	Body   []Stmt
	Orelse []Stmt
}

func (*While) stmtNode() {}
func (s *While) pos() Pos { return s.P }

// For is `for target in iter: body` with optional else.
type For struct {
	P      Pos
	Target Expr
	Iter   Expr
	Body   []Stmt
	Orelse []Stmt
}

func (*For) stmtNode() {}
func (s *For) pos() Pos { return s.P }

// AsyncFor is `async for target in iter: body`.
type AsyncFor struct {
	P      Pos
	Target Expr
	Iter   Expr
	Body   []Stmt
	Orelse []Stmt
}

func (*AsyncFor) stmtNode() {}
func (s *AsyncFor) pos() Pos { return s.P }

// ExceptHandler is one `except [Type [as Name]]: body` clause.
type ExceptHandler struct {
	P    Pos
	Type Expr   // nil for bare except
	Name string // empty for no `as N`
	Body []Stmt
}

// Try is the try/except/else/finally compound statement.
type Try struct {
	P         Pos
	Body      []Stmt
	Handlers  []*ExceptHandler
	Orelse    []Stmt
	Finalbody []Stmt
}

func (*Try) stmtNode() {}
func (s *Try) pos() Pos { return s.P }

// WithItem is one `expr as target` clause inside a with statement.
type WithItem struct {
	P             Pos
	ContextExpr   Expr
	OptionalVars  Expr // nil when no `as N`
}

// With is `with item, item, ...: body`.
type With struct {
	P     Pos
	Items []*WithItem
	Body  []Stmt
}

func (*With) stmtNode() {}
func (s *With) pos() Pos { return s.P }

// AsyncWith is `async with item, ...: body`.
type AsyncWith struct {
	P     Pos
	Items []*WithItem
	Body  []Stmt
}

func (*AsyncWith) stmtNode() {}
func (s *AsyncWith) pos() Pos { return s.P }

// FunctionDef is `def name(args) -> ret: body` with decorators.
type FunctionDef struct {
	P             Pos
	Name          string
	Args          *Arguments
	Body          []Stmt
	DecoratorList []Expr
	Returns       Expr
}

func (*FunctionDef) stmtNode() {}
func (s *FunctionDef) pos() Pos { return s.P }

// AsyncFunctionDef is `async def name(args): body`.
type AsyncFunctionDef struct {
	P             Pos
	Name          string
	Args          *Arguments
	Body          []Stmt
	DecoratorList []Expr
	Returns       Expr
}

func (*AsyncFunctionDef) stmtNode() {}
func (s *AsyncFunctionDef) pos() Pos { return s.P }

// ClassDef is `class Name(bases, kw=val): body` with decorators.
type ClassDef struct {
	P             Pos
	Name          string
	Bases         []Expr
	Keywords      []*Keyword
	Body          []Stmt
	DecoratorList []Expr
}

func (*ClassDef) stmtNode() {}
func (s *ClassDef) pos() Pos { return s.P }

// Dump returns a stable, single-line, parens-explicit textual
// representation of the tree. The format mirrors CPython's `ast.dump`
// well enough that v1 and v2 outputs can be diffed for parity tests.
func Dump(e Expr) string {
	var b strings.Builder
	dump(&b, e)
	return b.String()
}

// DumpModule returns a single-line ast.dump-style rendering of a
// Module node. Bodies are recursive; statement nodes use the same
// dumping conventions as Expr.
func DumpModule(m *Module) string {
	var b strings.Builder
	b.WriteString("Module(body=[")
	for i, s := range m.Body {
		if i > 0 {
			b.WriteString(", ")
		}
		dumpStmt(&b, s)
	}
	b.WriteString("])")
	return b.String()
}

func dumpStmt(b *strings.Builder, s Stmt) {
	if s == nil {
		b.WriteString("nil")
		return
	}
	switch n := s.(type) {
	case *ExprStmt:
		b.WriteString("Expr(value=")
		dump(b, n.Value)
		b.WriteString(")")
	case *Assign:
		b.WriteString("Assign(targets=[")
		for i, t := range n.Targets {
			if i > 0 {
				b.WriteString(", ")
			}
			dump(b, t)
		}
		b.WriteString("], value=")
		dump(b, n.Value)
		b.WriteString(")")
	case *AugAssign:
		fmt.Fprintf(b, "AugAssign(target=")
		dump(b, n.Target)
		fmt.Fprintf(b, ", op=%s, value=", n.Op)
		dump(b, n.Value)
		b.WriteString(")")
	case *AnnAssign:
		b.WriteString("AnnAssign(target=")
		dump(b, n.Target)
		b.WriteString(", annotation=")
		dump(b, n.Annotation)
		if n.Value != nil {
			b.WriteString(", value=")
			dump(b, n.Value)
		}
		fmt.Fprintf(b, ", simple=%v)", n.Simple)
	case *Return:
		b.WriteString("Return(")
		if n.Value != nil {
			b.WriteString("value=")
			dump(b, n.Value)
		}
		b.WriteString(")")
	case *Raise:
		b.WriteString("Raise(")
		if n.Exc != nil {
			b.WriteString("exc=")
			dump(b, n.Exc)
		}
		if n.Cause != nil {
			if n.Exc != nil {
				b.WriteString(", ")
			}
			b.WriteString("cause=")
			dump(b, n.Cause)
		}
		b.WriteString(")")
	case *Pass:
		b.WriteString("Pass()")
	case *Break:
		b.WriteString("Break()")
	case *Continue:
		b.WriteString("Continue()")
	case *Import:
		b.WriteString("Import(names=[")
		for i, a := range n.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			dumpAlias(b, a)
		}
		b.WriteString("])")
	case *ImportFrom:
		fmt.Fprintf(b, "ImportFrom(module=%q, names=[", n.Module)
		for i, a := range n.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			dumpAlias(b, a)
		}
		fmt.Fprintf(b, "], level=%d)", n.Level)
	case *Global:
		fmt.Fprintf(b, "Global(names=%q)", n.Names)
	case *Nonlocal:
		fmt.Fprintf(b, "Nonlocal(names=%q)", n.Names)
	case *Delete:
		b.WriteString("Delete(targets=[")
		for i, t := range n.Targets {
			if i > 0 {
				b.WriteString(", ")
			}
			dump(b, t)
		}
		b.WriteString("])")
	case *Assert:
		b.WriteString("Assert(test=")
		dump(b, n.Test)
		if n.Msg != nil {
			b.WriteString(", msg=")
			dump(b, n.Msg)
		}
		b.WriteString(")")
	case *If:
		b.WriteString("If(test=")
		dump(b, n.Test)
		b.WriteString(", body=")
		dumpStmtList(b, n.Body)
		b.WriteString(", orelse=")
		dumpStmtList(b, n.Orelse)
		b.WriteString(")")
	case *While:
		b.WriteString("While(test=")
		dump(b, n.Test)
		b.WriteString(", body=")
		dumpStmtList(b, n.Body)
		b.WriteString(", orelse=")
		dumpStmtList(b, n.Orelse)
		b.WriteString(")")
	case *For:
		b.WriteString("For(target=")
		dump(b, n.Target)
		b.WriteString(", iter=")
		dump(b, n.Iter)
		b.WriteString(", body=")
		dumpStmtList(b, n.Body)
		b.WriteString(", orelse=")
		dumpStmtList(b, n.Orelse)
		b.WriteString(")")
	case *AsyncFor:
		b.WriteString("AsyncFor(target=")
		dump(b, n.Target)
		b.WriteString(", iter=")
		dump(b, n.Iter)
		b.WriteString(", body=")
		dumpStmtList(b, n.Body)
		b.WriteString(", orelse=")
		dumpStmtList(b, n.Orelse)
		b.WriteString(")")
	case *Try:
		b.WriteString("Try(body=")
		dumpStmtList(b, n.Body)
		b.WriteString(", handlers=[")
		for i, h := range n.Handlers {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("ExceptHandler(type=")
			if h.Type != nil {
				dump(b, h.Type)
			} else {
				b.WriteString("nil")
			}
			fmt.Fprintf(b, ", name=%q, body=", h.Name)
			dumpStmtList(b, h.Body)
			b.WriteString(")")
		}
		b.WriteString("], orelse=")
		dumpStmtList(b, n.Orelse)
		b.WriteString(", finalbody=")
		dumpStmtList(b, n.Finalbody)
		b.WriteString(")")
	case *With:
		b.WriteString("With(items=[")
		dumpWithItems(b, n.Items)
		b.WriteString("], body=")
		dumpStmtList(b, n.Body)
		b.WriteString(")")
	case *AsyncWith:
		b.WriteString("AsyncWith(items=[")
		dumpWithItems(b, n.Items)
		b.WriteString("], body=")
		dumpStmtList(b, n.Body)
		b.WriteString(")")
	case *FunctionDef:
		fmt.Fprintf(b, "FunctionDef(name=%q, args=", n.Name)
		dumpArgs(b, n.Args)
		b.WriteString(", body=")
		dumpStmtList(b, n.Body)
		b.WriteString(", decorators=[")
		dumpList(b, n.DecoratorList)
		b.WriteString("], returns=")
		if n.Returns != nil {
			dump(b, n.Returns)
		} else {
			b.WriteString("nil")
		}
		b.WriteString(")")
	case *AsyncFunctionDef:
		fmt.Fprintf(b, "AsyncFunctionDef(name=%q, args=", n.Name)
		dumpArgs(b, n.Args)
		b.WriteString(", body=")
		dumpStmtList(b, n.Body)
		b.WriteString(", decorators=[")
		dumpList(b, n.DecoratorList)
		b.WriteString("], returns=")
		if n.Returns != nil {
			dump(b, n.Returns)
		} else {
			b.WriteString("nil")
		}
		b.WriteString(")")
	case *ClassDef:
		fmt.Fprintf(b, "ClassDef(name=%q, bases=[", n.Name)
		dumpList(b, n.Bases)
		b.WriteString("], keywords=[")
		for i, kw := range n.Keywords {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(b, "Keyword(arg=%q, value=", kw.Arg)
			dump(b, kw.Value)
			b.WriteString(")")
		}
		b.WriteString("], body=")
		dumpStmtList(b, n.Body)
		b.WriteString(", decorators=[")
		dumpList(b, n.DecoratorList)
		b.WriteString("])")
	default:
		fmt.Fprintf(b, "<unknown stmt %T>", n)
	}
}

func dumpStmtList(b *strings.Builder, ss []Stmt) {
	b.WriteString("[")
	for i, s := range ss {
		if i > 0 {
			b.WriteString(", ")
		}
		dumpStmt(b, s)
	}
	b.WriteString("]")
}

func dumpAlias(b *strings.Builder, a *Alias) {
	if a.Asname == "" {
		fmt.Fprintf(b, "Alias(name=%q)", a.Name)
		return
	}
	fmt.Fprintf(b, "Alias(name=%q, asname=%q)", a.Name, a.Asname)
}

func dumpWithItems(b *strings.Builder, items []*WithItem) {
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("WithItem(context=")
		dump(b, it.ContextExpr)
		if it.OptionalVars != nil {
			b.WriteString(", vars=")
			dump(b, it.OptionalVars)
		}
		b.WriteString(")")
	}
}

func dump(b *strings.Builder, e Expr) {
	if e == nil {
		b.WriteString("nil")
		return
	}
	switch n := e.(type) {
	case *Constant:
		fmt.Fprintf(b, "Constant(value=%s)", constRepr(n.Kind, n.Value))
	case *Name:
		fmt.Fprintf(b, "Name(id=%q)", n.Id)
	case *UnaryOp:
		fmt.Fprintf(b, "UnaryOp(op=%s, operand=", n.Op)
		dump(b, n.Operand)
		b.WriteString(")")
	case *BinOp:
		fmt.Fprintf(b, "BinOp(op=%s, left=", n.Op)
		dump(b, n.Left)
		b.WriteString(", right=")
		dump(b, n.Right)
		b.WriteString(")")
	case *BoolOp:
		fmt.Fprintf(b, "BoolOp(op=%s, values=[", n.Op)
		dumpList(b, n.Values)
		b.WriteString("])")
	case *Compare:
		b.WriteString("Compare(left=")
		dump(b, n.Left)
		b.WriteString(", ops=[")
		for i, op := range n.Ops {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(op)
		}
		b.WriteString("], comparators=[")
		dumpList(b, n.Comparators)
		b.WriteString("])")
	case *Attribute:
		b.WriteString("Attribute(value=")
		dump(b, n.Value)
		fmt.Fprintf(b, ", attr=%q)", n.Attr)
	case *Subscript:
		b.WriteString("Subscript(value=")
		dump(b, n.Value)
		b.WriteString(", slice=")
		dump(b, n.Slice)
		b.WriteString(")")
	case *Slice:
		b.WriteString("Slice(lower=")
		dump(b, n.Lower)
		b.WriteString(", upper=")
		dump(b, n.Upper)
		b.WriteString(", step=")
		dump(b, n.Step)
		b.WriteString(")")
	case *Call:
		b.WriteString("Call(func=")
		dump(b, n.Func)
		b.WriteString(", args=[")
		dumpList(b, n.Args)
		b.WriteString("], keywords=[")
		for i, kw := range n.Keywords {
			if i > 0 {
				b.WriteString(", ")
			}
			if kw.Arg == "" {
				b.WriteString("**")
			} else {
				fmt.Fprintf(b, "%s=", kw.Arg)
			}
			dump(b, kw.Value)
		}
		b.WriteString("])")
	case *List:
		b.WriteString("List([")
		dumpList(b, n.Elts)
		b.WriteString("])")
	case *Tuple:
		b.WriteString("Tuple([")
		dumpList(b, n.Elts)
		b.WriteString("])")
	case *Set:
		b.WriteString("Set([")
		dumpList(b, n.Elts)
		b.WriteString("])")
	case *Dict:
		b.WriteString("Dict(keys=[")
		dumpList(b, n.Keys)
		b.WriteString("], values=[")
		dumpList(b, n.Values)
		b.WriteString("])")
	case *ListComp:
		b.WriteString("ListComp(elt=")
		dump(b, n.Elt)
		dumpComps(b, n.Gens)
	case *SetComp:
		b.WriteString("SetComp(elt=")
		dump(b, n.Elt)
		dumpComps(b, n.Gens)
	case *DictComp:
		b.WriteString("DictComp(key=")
		dump(b, n.Key)
		b.WriteString(", value=")
		dump(b, n.Value)
		dumpComps(b, n.Gens)
	case *GeneratorExp:
		b.WriteString("GeneratorExp(elt=")
		dump(b, n.Elt)
		dumpComps(b, n.Gens)
	case *Lambda:
		b.WriteString("Lambda(args=")
		dumpArgs(b, n.Args)
		b.WriteString(", body=")
		dump(b, n.Body)
		b.WriteString(")")
	case *NamedExpr:
		b.WriteString("NamedExpr(target=")
		dump(b, n.Target)
		b.WriteString(", value=")
		dump(b, n.Value)
		b.WriteString(")")
	case *Starred:
		b.WriteString("Starred(value=")
		dump(b, n.Value)
		b.WriteString(")")
	case *IfExp:
		b.WriteString("IfExp(test=")
		dump(b, n.Test)
		b.WriteString(", body=")
		dump(b, n.Body)
		b.WriteString(", orelse=")
		dump(b, n.OrElse)
		b.WriteString(")")
	case *Await:
		b.WriteString("Await(value=")
		dump(b, n.Value)
		b.WriteString(")")
	case *Yield:
		if n.Value == nil {
			b.WriteString("Yield()")
		} else {
			b.WriteString("Yield(value=")
			dump(b, n.Value)
			b.WriteString(")")
		}
	case *YieldFrom:
		b.WriteString("YieldFrom(value=")
		dump(b, n.Value)
		b.WriteString(")")
	case *JoinedStr:
		b.WriteString("JoinedStr(values=[")
		for i, v := range n.Values {
			if i > 0 {
				b.WriteString(", ")
			}
			dump(b, v)
		}
		b.WriteString("])")
	case *FormattedValue:
		b.WriteString("FormattedValue(value=")
		dump(b, n.Value)
		if n.Conversion != -1 {
			fmt.Fprintf(b, ", conversion=%d", n.Conversion)
		}
		if n.FormatSpec != nil {
			b.WriteString(", format_spec=")
			dump(b, n.FormatSpec)
		}
		b.WriteString(")")
	case *TemplateStr:
		b.WriteString("TemplateStr(strings=[")
		for i, c := range n.Strings {
			if i > 0 {
				b.WriteString(", ")
			}
			dump(b, c)
		}
		b.WriteString("], interpolations=[")
		for i, ip := range n.Interpolations {
			if i > 0 {
				b.WriteString(", ")
			}
			dump(b, ip)
		}
		b.WriteString("])")
	case *Interpolation:
		b.WriteString("Interpolation(value=")
		dump(b, n.Value)
		fmt.Fprintf(b, ", str=%q", n.Str)
		if n.Conversion != -1 {
			fmt.Fprintf(b, ", conversion=%d", n.Conversion)
		}
		if n.FormatSpec != nil {
			b.WriteString(", format_spec=")
			dump(b, n.FormatSpec)
		}
		b.WriteString(")")
	default:
		fmt.Fprintf(b, "<unknown:%T>", e)
	}
}

func dumpList(b *strings.Builder, es []Expr) {
	for i, e := range es {
		if i > 0 {
			b.WriteString(", ")
		}
		dump(b, e)
	}
}

func dumpComps(b *strings.Builder, gens []*Comprehension) {
	for _, g := range gens {
		b.WriteString(", for(")
		if g.IsAsync {
			b.WriteString("async ")
		}
		b.WriteString("target=")
		dump(b, g.Target)
		b.WriteString(", iter=")
		dump(b, g.Iter)
		if len(g.Ifs) > 0 {
			b.WriteString(", ifs=[")
			dumpList(b, g.Ifs)
			b.WriteString("]")
		}
		b.WriteString(")")
	}
	b.WriteString(")")
}

func dumpArgs(b *strings.Builder, a *Arguments) {
	if a == nil {
		b.WriteString("nil")
		return
	}
	b.WriteString("Arguments(")
	if len(a.PosOnly) > 0 {
		b.WriteString("posonly=[")
		for i, p := range a.PosOnly {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
		}
		b.WriteString("], ")
	}
	b.WriteString("args=[")
	for i, p := range a.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
	}
	b.WriteString("]")
	if a.Vararg != nil {
		fmt.Fprintf(b, ", vararg=%s", a.Vararg.Name)
	}
	if len(a.KwOnly) > 0 {
		b.WriteString(", kwonly=[")
		for i, p := range a.KwOnly {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
		}
		b.WriteString("]")
	}
	if a.Kwarg != nil {
		fmt.Fprintf(b, ", kwarg=%s", a.Kwarg.Name)
	}
	b.WriteString(")")
}

func constRepr(kind string, v any) string {
	switch kind {
	case "str":
		return fmt.Sprintf("%q", v)
	case "bytes":
		return fmt.Sprintf("b%q", v)
	case "None":
		return "None"
	case "True":
		return "True"
	case "False":
		return "False"
	case "Ellipsis":
		return "..."
	default:
		return fmt.Sprintf("%v", v)
	}
}
