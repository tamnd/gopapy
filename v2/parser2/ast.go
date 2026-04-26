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

// Expr is the closed sum type for parser2 expression nodes. The
// concrete types mirror v1's ast.ExprNode shapes (Constant, Name,
// UnaryOp, BinOp) so a future converter is mechanical. v2 keeps its
// own copies because Go's strict major-version-suffix rule prevents
// v2 from importing v1 directly while v1 ships under the /v1 suffix
// with v0.x.x tags.
type Expr interface {
	exprNode()
	pos() Pos
}

// Constant is an int, float, string, None, True, False, or Ellipsis
// literal. Value is the Go-typed payload; Kind names the source form
// for the dump representation.
type Constant struct {
	P     Pos
	Kind  string
	Value any
}

func (*Constant) exprNode() {}
func (c *Constant) pos() Pos { return c.P }

// Name is a bare identifier reference.
type Name struct {
	P  Pos
	Id string
}

func (*Name) exprNode() {}
func (n *Name) pos() Pos { return n.P }

// UnaryOp is a prefix operator application.
type UnaryOp struct {
	P       Pos
	Op      string
	Operand Expr
}

func (*UnaryOp) exprNode() {}
func (u *UnaryOp) pos() Pos { return u.P }

// BinOp is a binary arithmetic operator application.
type BinOp struct {
	P     Pos
	Op    string
	Left  Expr
	Right Expr
}

func (*BinOp) exprNode() {}
func (b *BinOp) pos() Pos { return b.P }

// Dump returns a stable, single-line, parens-explicit textual
// representation of the tree. The format mirrors CPython's `ast.dump`
// well enough that v1 and v2 outputs can be diffed for parity tests.
func Dump(e Expr) string {
	var b strings.Builder
	dump(&b, e)
	return b.String()
}

func dump(b *strings.Builder, e Expr) {
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
	default:
		fmt.Fprintf(b, "<unknown:%T>", e)
	}
}

func constRepr(kind string, v any) string {
	switch kind {
	case "str":
		return fmt.Sprintf("%q", v)
	case "None":
		return "None"
	case "True":
		return "True"
	case "False":
		return "False"
	default:
		return fmt.Sprintf("%v", v)
	}
}
