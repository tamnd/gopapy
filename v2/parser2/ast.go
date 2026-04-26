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

// IfExp is the conditional expression `body if test else orelse`.
type IfExp struct {
	P      Pos
	Test   Expr
	Body   Expr
	OrElse Expr
}

func (*IfExp) exprNode()  {}
func (n *IfExp) pos() Pos { return n.P }

// Dump returns a stable, single-line, parens-explicit textual
// representation of the tree. The format mirrors CPython's `ast.dump`
// well enough that v1 and v2 outputs can be diffed for parity tests.
func Dump(e Expr) string {
	var b strings.Builder
	dump(&b, e)
	return b.String()
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
