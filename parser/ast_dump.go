package parser

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

// ASTDump returns a single-line CPython ast.dump-style rendering of m.
// The output is byte-identical to Python's ast.dump(ast.parse(src)) for
// all well-formed modules.
func ASTDump(m *Module) string {
	var b strings.Builder
	b.WriteString("Module(body=[")
	for i, s := range m.Body {
		if i > 0 {
			b.WriteString(", ")
		}
		adStmt(&b, s)
	}
	b.WriteString("])")
	return b.String()
}

// pyRepr returns a Python repr-style quoted string (single-quote preferred).
func pyRepr(s string) string {
	hasSingle := strings.ContainsRune(s, '\'')
	hasDouble := strings.ContainsRune(s, '"')
	useDouble := hasSingle && !hasDouble

	var b strings.Builder
	if useDouble {
		b.WriteByte('"')
	} else {
		b.WriteByte('\'')
	}
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\'' && !useDouble:
			b.WriteString(`\'`)
		case r == '"' && useDouble:
			b.WriteString(`\"`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r > 0x7f && !unicode.IsPrint(r):
			if r <= 0xff {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else if r <= 0xffff {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				fmt.Fprintf(&b, `\U%08x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	if useDouble {
		b.WriteByte('"')
	} else {
		b.WriteByte('\'')
	}
	return b.String()
}

// pyBytesRepr returns a Python repr-style bytes literal b'...'.
func pyBytesRepr(bs []byte) string {
	hasSingle := false
	hasDouble := false
	for _, c := range bs {
		if c == '\'' {
			hasSingle = true
		}
		if c == '"' {
			hasDouble = true
		}
	}
	useDouble := hasSingle && !hasDouble

	var b strings.Builder
	b.WriteByte('b')
	if useDouble {
		b.WriteByte('"')
	} else {
		b.WriteByte('\'')
	}
	for _, c := range bs {
		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\'' && !useDouble:
			b.WriteString(`\'`)
		case c == '"' && useDouble:
			b.WriteString(`\"`)
		case c < 0x20 || c >= 0x7f:
			fmt.Fprintf(&b, `\x%02x`, c)
		default:
			b.WriteByte(c)
		}
	}
	if useDouble {
		b.WriteByte('"')
	} else {
		b.WriteByte('\'')
	}
	return b.String()
}

// pyFloat formats a float64 to match Python's repr() for floats.
// Python uses decimal notation for values in [1e-4, 1e16) and
// scientific notation otherwise.
func pyFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	if math.IsNaN(f) {
		return "nan"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	abs := math.Abs(f)
	if abs >= 1e-4 && abs < 1e16 {
		// Decimal form: use 'f' format (shortest that round-trips).
		s := strconv.FormatFloat(f, 'f', -1, 64)
		if !strings.ContainsRune(s, '.') {
			s += ".0"
		}
		return s
	}
	// Scientific notation: Go's 'e' format matches Python's repr.
	return strconv.FormatFloat(f, 'e', -1, 64)
}

// adConstValue writes the Python repr of a Constant node's value.
func adConstValue(b *strings.Builder, kind string, v any) {
	switch kind {
	case "None":
		b.WriteString("None")
	case "True":
		b.WriteString("True")
	case "False":
		b.WriteString("False")
	case "Ellipsis":
		b.WriteString("Ellipsis")
	case "str", "u":
		if s, ok := v.(string); ok {
			b.WriteString(pyRepr(s))
		}
	case "bytes":
		switch bv := v.(type) {
		case []byte:
			b.WriteString(pyBytesRepr(bv))
		case string:
			b.WriteString(pyBytesRepr([]byte(bv)))
		}
	case "float":
		switch fv := v.(type) {
		case float64:
			b.WriteString(pyFloat(fv))
		case float32:
			b.WriteString(pyFloat(float64(fv)))
		}
	case "complex":
		// Parser2 stores complex literals as a string like "1j" or "2.5j".
		if s, ok := v.(string); ok {
			s = strings.TrimRight(s, "jJ")
			f, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
			// ErrRange means overflow (+Inf) or underflow (0.0) — both valid.
			if err == nil || errors.Is(err, strconv.ErrRange) {
				if math.IsInf(f, 1) {
					b.WriteString("inf")
				} else if math.IsInf(f, -1) {
					b.WriteString("-inf")
				} else {
					b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
				}
			} else {
				b.WriteString(s)
			}
			b.WriteByte('j')
		} else if c, ok := v.(complex128); ok {
			im := imag(c)
			b.WriteString(strconv.FormatFloat(im, 'g', -1, 64))
			b.WriteByte('j')
		}
	default:
		// *big.Int prints as decimal via its String() method, which matches
		// CPython's repr() for large integer constants.
		if bi, ok := v.(*big.Int); ok {
			b.WriteString(bi.String())
		} else {
			fmt.Fprintf(b, "%v", v)
		}
	}
}

// adOp writes an operator name with parentheses, e.g. "Add()".
func adOp(b *strings.Builder, op string) {
	b.WriteString(op)
	b.WriteString("()")
}

// adCtx writes ", ctx=Load()" etc.
func adCtx(b *strings.Builder, ctx string) {
	b.WriteString(", ctx=")
	b.WriteString(ctx)
	b.WriteString("()")
}

func adStmtList(b *strings.Builder, ss []Stmt) {
	b.WriteByte('[')
	for i, s := range ss {
		if i > 0 {
			b.WriteString(", ")
		}
		adStmt(b, s)
	}
	b.WriteByte(']')
}

func adExprList(b *strings.Builder, es []Expr, ctx string) {
	for i, e := range es {
		if i > 0 {
			b.WriteString(", ")
		}
		adExpr(b, e, ctx)
	}
}

func adStmt(b *strings.Builder, s Stmt) {
	if s == nil {
		b.WriteString("None")
		return
	}
	switch n := s.(type) {
	case *ExprStmt:
		b.WriteString("Expr(value=")
		adExpr(b, n.Value, "Load")
		b.WriteByte(')')

	case *Assign:
		b.WriteString("Assign(targets=[")
		for i, t := range n.Targets {
			if i > 0 {
				b.WriteString(", ")
			}
			adExpr(b, t, "Store")
		}
		b.WriteString("], value=")
		adExpr(b, n.Value, "Load")
		b.WriteByte(')')

	case *AugAssign:
		b.WriteString("AugAssign(target=")
		adExpr(b, n.Target, "Store")
		b.WriteString(", op=")
		adOp(b, n.Op)
		b.WriteString(", value=")
		adExpr(b, n.Value, "Load")
		b.WriteByte(')')

	case *AnnAssign:
		b.WriteString("AnnAssign(target=")
		adExpr(b, n.Target, "Store")
		b.WriteString(", annotation=")
		adExpr(b, n.Annotation, "Load")
		if n.Value != nil {
			b.WriteString(", value=")
			adExpr(b, n.Value, "Load")
		}
		if n.Simple {
			b.WriteString(", simple=1)")
		} else {
			b.WriteString(", simple=0)")
		}

	case *Return:
		if n.Value != nil {
			b.WriteString("Return(value=")
			adExpr(b, n.Value, "Load")
			b.WriteByte(')')
		} else {
			b.WriteString("Return()")
		}

	case *Delete:
		b.WriteString("Delete(targets=[")
		for i, t := range n.Targets {
			if i > 0 {
				b.WriteString(", ")
			}
			adExpr(b, t, "Del")
		}
		b.WriteString("])")

	case *Pass:
		b.WriteString("Pass()")
	case *Break:
		b.WriteString("Break()")
	case *Continue:
		b.WriteString("Continue()")

	case *Raise:
		if n.Exc == nil {
			b.WriteString("Raise()")
		} else {
			b.WriteString("Raise(exc=")
			adExpr(b, n.Exc, "Load")
			if n.Cause != nil {
				b.WriteString(", cause=")
				adExpr(b, n.Cause, "Load")
			}
			b.WriteByte(')')
		}

	case *Assert:
		b.WriteString("Assert(test=")
		adExpr(b, n.Test, "Load")
		if n.Msg != nil {
			b.WriteString(", msg=")
			adExpr(b, n.Msg, "Load")
		}
		b.WriteByte(')')

	case *Global:
		b.WriteString("Global(names=[")
		for i, name := range n.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(pyRepr(name))
		}
		b.WriteString("])")

	case *Nonlocal:
		b.WriteString("Nonlocal(names=[")
		for i, name := range n.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(pyRepr(name))
		}
		b.WriteString("])")

	case *Import:
		b.WriteString("Import(names=[")
		for i, a := range n.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			adAlias(b, a)
		}
		b.WriteString("])")

	case *ImportFrom:
		b.WriteString("ImportFrom(")
		if n.Module != "" {
			b.WriteString("module=")
			b.WriteString(pyRepr(n.Module))
			b.WriteString(", ")
		}
		b.WriteString("names=[")
		for i, a := range n.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			adAlias(b, a)
		}
		fmt.Fprintf(b, "], level=%d)", n.Level)

	case *If:
		b.WriteString("If(test=")
		adExpr(b, n.Test, "Load")
		b.WriteString(", body=")
		adStmtList(b, n.Body)
		if len(n.Orelse) > 0 {
			b.WriteString(", orelse=")
			adStmtList(b, n.Orelse)
		}
		b.WriteByte(')')

	case *While:
		b.WriteString("While(test=")
		adExpr(b, n.Test, "Load")
		b.WriteString(", body=")
		adStmtList(b, n.Body)
		if len(n.Orelse) > 0 {
			b.WriteString(", orelse=")
			adStmtList(b, n.Orelse)
		}
		b.WriteByte(')')

	case *For:
		b.WriteString("For(target=")
		adExpr(b, n.Target, "Store")
		b.WriteString(", iter=")
		adExpr(b, n.Iter, "Load")
		b.WriteString(", body=")
		adStmtList(b, n.Body)
		if len(n.Orelse) > 0 {
			b.WriteString(", orelse=")
			adStmtList(b, n.Orelse)
		}
		b.WriteByte(')')

	case *AsyncFor:
		b.WriteString("AsyncFor(target=")
		adExpr(b, n.Target, "Store")
		b.WriteString(", iter=")
		adExpr(b, n.Iter, "Load")
		b.WriteString(", body=")
		adStmtList(b, n.Body)
		if len(n.Orelse) > 0 {
			b.WriteString(", orelse=")
			adStmtList(b, n.Orelse)
		}
		b.WriteByte(')')

	case *With:
		b.WriteString("With(items=[")
		adWithItems(b, n.Items)
		b.WriteString("], body=")
		adStmtList(b, n.Body)
		b.WriteByte(')')

	case *AsyncWith:
		b.WriteString("AsyncWith(items=[")
		adWithItems(b, n.Items)
		b.WriteString("], body=")
		adStmtList(b, n.Body)
		b.WriteByte(')')

	case *Try:
		b.WriteString("Try(body=")
		adStmtList(b, n.Body)
		b.WriteString(", handlers=[")
		for i, h := range n.Handlers {
			if i > 0 {
				b.WriteString(", ")
			}
			adExceptHandler(b, h)
		}
		b.WriteByte(']')
		if len(n.Orelse) > 0 {
			b.WriteString(", orelse=")
			adStmtList(b, n.Orelse)
		}
		if len(n.Finalbody) > 0 {
			b.WriteString(", finalbody=")
			adStmtList(b, n.Finalbody)
		}
		b.WriteByte(')')

	case *TryStar:
		b.WriteString("TryStar(body=")
		adStmtList(b, n.Body)
		b.WriteString(", handlers=[")
		for i, h := range n.Handlers {
			if i > 0 {
				b.WriteString(", ")
			}
			adExceptHandler(b, h)
		}
		b.WriteByte(']')
		if len(n.Orelse) > 0 {
			b.WriteString(", orelse=")
			adStmtList(b, n.Orelse)
		}
		if len(n.Finalbody) > 0 {
			b.WriteString(", finalbody=")
			adStmtList(b, n.Finalbody)
		}
		b.WriteByte(')')

	case *FunctionDef:
		b.WriteString("FunctionDef(name=")
		b.WriteString(pyRepr(n.Name))
		b.WriteString(", args=")
		adArgs(b, n.Args)
		b.WriteString(", body=")
		adStmtList(b, n.Body)
		if len(n.DecoratorList) > 0 {
			b.WriteString(", decorator_list=[")
			adExprList(b, n.DecoratorList, "Load")
			b.WriteByte(']')
		}
		if n.Returns != nil {
			b.WriteString(", returns=")
			adExpr(b, n.Returns, "Load")
		}
		if len(n.TypeParams) > 0 {
			b.WriteString(", type_params=[")
			adTypeParams(b, n.TypeParams)
			b.WriteByte(']')
		}
		b.WriteByte(')')

	case *AsyncFunctionDef:
		b.WriteString("AsyncFunctionDef(name=")
		b.WriteString(pyRepr(n.Name))
		b.WriteString(", args=")
		adArgs(b, n.Args)
		b.WriteString(", body=")
		adStmtList(b, n.Body)
		if len(n.DecoratorList) > 0 {
			b.WriteString(", decorator_list=[")
			adExprList(b, n.DecoratorList, "Load")
			b.WriteByte(']')
		}
		if n.Returns != nil {
			b.WriteString(", returns=")
			adExpr(b, n.Returns, "Load")
		}
		if len(n.TypeParams) > 0 {
			b.WriteString(", type_params=[")
			adTypeParams(b, n.TypeParams)
			b.WriteByte(']')
		}
		b.WriteByte(')')

	case *ClassDef:
		b.WriteString("ClassDef(name=")
		b.WriteString(pyRepr(n.Name))
		if len(n.Bases) > 0 {
			b.WriteString(", bases=[")
			adExprList(b, n.Bases, "Load")
			b.WriteByte(']')
		}
		if len(n.Keywords) > 0 {
			b.WriteString(", keywords=[")
			for i, kw := range n.Keywords {
				if i > 0 {
					b.WriteString(", ")
				}
				adKeyword(b, kw)
			}
			b.WriteByte(']')
		}
		b.WriteString(", body=")
		adStmtList(b, n.Body)
		if len(n.DecoratorList) > 0 {
			b.WriteString(", decorator_list=[")
			adExprList(b, n.DecoratorList, "Load")
			b.WriteByte(']')
		}
		if len(n.TypeParams) > 0 {
			b.WriteString(", type_params=[")
			adTypeParams(b, n.TypeParams)
			b.WriteByte(']')
		}
		b.WriteByte(')')

	case *TypeAlias:
		b.WriteString("TypeAlias(name=")
		adExpr(b, n.Name, "Store")
		if len(n.TypeParams) > 0 {
			b.WriteString(", type_params=[")
			adTypeParams(b, n.TypeParams)
			b.WriteByte(']')
		}
		b.WriteString(", value=")
		adExpr(b, n.Value, "Load")
		b.WriteByte(')')

	case *Match:
		b.WriteString("Match(subject=")
		adExpr(b, n.Subject, "Load")
		b.WriteString(", cases=[")
		for i, c := range n.Cases {
			if i > 0 {
				b.WriteString(", ")
			}
			adMatchCase(b, c)
		}
		b.WriteString("])")

	default:
		fmt.Fprintf(b, "<unknown stmt %T>", s)
	}
}

func adExpr(b *strings.Builder, e Expr, ctx string) {
	if e == nil {
		b.WriteString("None")
		return
	}
	switch n := e.(type) {
	case *Constant:
		b.WriteString("Constant(value=")
		adConstValue(b, n.Kind, n.Value)
		if n.Kind == "u" {
			b.WriteString(", kind='u'")
		}
		b.WriteByte(')')

	case *Name:
		b.WriteString("Name(id=")
		b.WriteString(pyRepr(n.Id))
		adCtx(b, ctx)
		b.WriteByte(')')

	case *Attribute:
		b.WriteString("Attribute(value=")
		adExpr(b, n.Value, "Load")
		b.WriteString(", attr=")
		b.WriteString(pyRepr(n.Attr))
		adCtx(b, ctx)
		b.WriteByte(')')

	case *Subscript:
		b.WriteString("Subscript(value=")
		adExpr(b, n.Value, "Load")
		b.WriteString(", slice=")
		adExpr(b, n.Slice, "Load")
		adCtx(b, ctx)
		b.WriteByte(')')

	case *Starred:
		b.WriteString("Starred(value=")
		adExpr(b, n.Value, ctx)
		adCtx(b, ctx)
		b.WriteByte(')')

	case *List:
		b.WriteString("List(")
		if len(n.Elts) > 0 {
			b.WriteString("elts=[")
			adExprList(b, n.Elts, ctx)
			b.WriteString("], ctx=")
			b.WriteString(ctx)
			b.WriteString("())")
		} else {
			b.WriteString("ctx=")
			b.WriteString(ctx)
			b.WriteString("())")
		}

	case *Tuple:
		b.WriteString("Tuple(")
		if len(n.Elts) > 0 {
			b.WriteString("elts=[")
			adExprList(b, n.Elts, ctx)
			b.WriteString("], ctx=")
			b.WriteString(ctx)
			b.WriteString("())")
		} else {
			b.WriteString("ctx=")
			b.WriteString(ctx)
			b.WriteString("())")
		}

	case *Set:
		b.WriteString("Set(elts=[")
		adExprList(b, n.Elts, "Load")
		b.WriteString("])")

	case *Dict:
		b.WriteString("Dict(keys=[")
		for i, k := range n.Keys {
			if i > 0 {
				b.WriteString(", ")
			}
			if k == nil {
				b.WriteString("None")
			} else {
				adExpr(b, k, "Load")
			}
		}
		b.WriteString("], values=[")
		adExprList(b, n.Values, "Load")
		b.WriteString("])")

	case *BinOp:
		b.WriteString("BinOp(left=")
		adExpr(b, n.Left, "Load")
		b.WriteString(", op=")
		adOp(b, n.Op)
		b.WriteString(", right=")
		adExpr(b, n.Right, "Load")
		b.WriteByte(')')

	case *UnaryOp:
		b.WriteString("UnaryOp(op=")
		adOp(b, n.Op)
		b.WriteString(", operand=")
		adExpr(b, n.Operand, "Load")
		b.WriteByte(')')

	case *BoolOp:
		b.WriteString("BoolOp(op=")
		adOp(b, n.Op)
		b.WriteString(", values=[")
		adExprList(b, n.Values, "Load")
		b.WriteString("])")

	case *Compare:
		b.WriteString("Compare(left=")
		adExpr(b, n.Left, "Load")
		b.WriteString(", ops=[")
		for i, op := range n.Ops {
			if i > 0 {
				b.WriteString(", ")
			}
			adOp(b, op)
		}
		b.WriteString("], comparators=[")
		adExprList(b, n.Comparators, "Load")
		b.WriteString("])")

	case *Call:
		b.WriteString("Call(func=")
		adExpr(b, n.Func, "Load")
		if len(n.Args) > 0 {
			b.WriteString(", args=[")
			adExprList(b, n.Args, "Load")
			b.WriteByte(']')
		}
		if len(n.Keywords) > 0 {
			b.WriteString(", keywords=[")
			for i, kw := range n.Keywords {
				if i > 0 {
					b.WriteString(", ")
				}
				adKeyword(b, kw)
			}
			b.WriteByte(']')
		}
		b.WriteByte(')')

	case *IfExp:
		b.WriteString("IfExp(test=")
		adExpr(b, n.Test, "Load")
		b.WriteString(", body=")
		adExpr(b, n.Body, "Load")
		b.WriteString(", orelse=")
		adExpr(b, n.OrElse, "Load")
		b.WriteByte(')')

	case *Lambda:
		b.WriteString("Lambda(args=")
		adArgs(b, n.Args)
		b.WriteString(", body=")
		adExpr(b, n.Body, "Load")
		b.WriteByte(')')

	case *NamedExpr:
		b.WriteString("NamedExpr(target=")
		adExpr(b, n.Target, "Store")
		b.WriteString(", value=")
		adExpr(b, n.Value, "Load")
		b.WriteByte(')')

	case *Await:
		b.WriteString("Await(value=")
		adExpr(b, n.Value, "Load")
		b.WriteByte(')')

	case *Yield:
		if n.Value != nil {
			b.WriteString("Yield(value=")
			adExpr(b, n.Value, "Load")
			b.WriteByte(')')
		} else {
			b.WriteString("Yield()")
		}

	case *YieldFrom:
		b.WriteString("YieldFrom(value=")
		adExpr(b, n.Value, "Load")
		b.WriteByte(')')

	case *JoinedStr:
		if len(n.Values) == 0 {
			b.WriteString("JoinedStr()")
		} else {
			b.WriteString("JoinedStr(values=[")
			for i, v := range n.Values {
				if i > 0 {
					b.WriteString(", ")
				}
				adExpr(b, v, "Load")
			}
			b.WriteString("])")
		}

	case *FormattedValue:
		b.WriteString("FormattedValue(value=")
		adExpr(b, n.Value, "Load")
		fmt.Fprintf(b, ", conversion=%d", n.Conversion)
		if n.FormatSpec != nil {
			b.WriteString(", format_spec=")
			adExpr(b, n.FormatSpec, "Load")
		}
		b.WriteByte(')')

	case *TemplateStr:
		// CPython 3.14 emits a flat values=[...] list, interleaving
		// string constants and interpolations in source order.
		// Empty string constants ("") are omitted from the flat list.
		type flatItem struct {
			str    *Constant
			interp *Interpolation
		}
		var flat []flatItem
		nInterp := len(n.Interpolations)
		for i, c := range n.Strings {
			if c.Value.(string) != "" {
				flat = append(flat, flatItem{str: c})
			}
			if i < nInterp {
				flat = append(flat, flatItem{interp: n.Interpolations[i]})
			}
		}
		if len(flat) == 0 {
			b.WriteString("TemplateStr()")
		} else {
			b.WriteString("TemplateStr(values=[")
			for i, it := range flat {
				if i > 0 {
					b.WriteString(", ")
				}
				if it.str != nil {
					adExpr(b, it.str, "Load")
				} else {
					adExpr(b, it.interp, "Load")
				}
			}
			b.WriteString("])")
		}

	case *Interpolation:
		b.WriteString("Interpolation(value=")
		adExpr(b, n.Value, "Load")
		b.WriteString(", str=")
		b.WriteString(pyRepr(n.Str))
		fmt.Fprintf(b, ", conversion=%d", n.Conversion)
		if n.FormatSpec != nil {
			b.WriteString(", format_spec=")
			adExpr(b, n.FormatSpec, "Load")
		}
		b.WriteByte(')')

	case *Slice:
		first := true
		buf := &strings.Builder{}
		if n.Lower != nil {
			buf.WriteString("lower=")
			adExpr(buf, n.Lower, "Load")
			first = false
		}
		if n.Upper != nil {
			if !first {
				buf.WriteString(", ")
			}
			buf.WriteString("upper=")
			adExpr(buf, n.Upper, "Load")
			first = false
		}
		if n.Step != nil {
			if !first {
				buf.WriteString(", ")
			}
			buf.WriteString("step=")
			adExpr(buf, n.Step, "Load")
		}
		b.WriteString("Slice(")
		b.WriteString(buf.String())
		b.WriteByte(')')

	case *ListComp:
		b.WriteString("ListComp(elt=")
		adExpr(b, n.Elt, "Load")
		b.WriteString(", generators=[")
		adComps(b, n.Gens)
		b.WriteString("])")

	case *SetComp:
		b.WriteString("SetComp(elt=")
		adExpr(b, n.Elt, "Load")
		b.WriteString(", generators=[")
		adComps(b, n.Gens)
		b.WriteString("])")

	case *DictComp:
		b.WriteString("DictComp(key=")
		adExpr(b, n.Key, "Load")
		b.WriteString(", value=")
		adExpr(b, n.Value, "Load")
		b.WriteString(", generators=[")
		adComps(b, n.Gens)
		b.WriteString("])")

	case *GeneratorExp:
		b.WriteString("GeneratorExp(elt=")
		adExpr(b, n.Elt, "Load")
		b.WriteString(", generators=[")
		adComps(b, n.Gens)
		b.WriteString("])")

	default:
		fmt.Fprintf(b, "<unknown expr %T>", e)
	}
}

// adArgs writes an arguments(...) node.
// Fields are omitted when empty/None, matching CPython 3.14 behavior.
func adArgs(b *strings.Builder, a *Arguments) {
	if a == nil {
		b.WriteString("arguments()")
		return
	}
	b.WriteString("arguments(")
	sep := false
	if len(a.PosOnly) > 0 {
		b.WriteString("posonlyargs=[")
		for i, p := range a.PosOnly {
			if i > 0 {
				b.WriteString(", ")
			}
			adArg(b, p)
		}
		b.WriteByte(']')
		sep = true
	}
	if len(a.Args) > 0 {
		if sep {
			b.WriteString(", ")
		}
		b.WriteString("args=[")
		for i, p := range a.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			adArg(b, p)
		}
		b.WriteByte(']')
		sep = true
	}
	if a.Vararg != nil {
		if sep {
			b.WriteString(", ")
		}
		b.WriteString("vararg=")
		adArg(b, a.Vararg)
		sep = true
	}
	if len(a.KwOnly) > 0 {
		if sep {
			b.WriteString(", ")
		}
		b.WriteString("kwonlyargs=[")
		for i, p := range a.KwOnly {
			if i > 0 {
				b.WriteString(", ")
			}
			adArg(b, p)
		}
		b.WriteByte(']')
		sep = true
	}
	if len(a.KwOnlyDef) > 0 {
		if sep {
			b.WriteString(", ")
		}
		b.WriteString("kw_defaults=[")
		for i, d := range a.KwOnlyDef {
			if i > 0 {
				b.WriteString(", ")
			}
			if d == nil {
				b.WriteString("None")
			} else {
				adExpr(b, d, "Load")
			}
		}
		b.WriteByte(']')
		sep = true
	}
	if a.Kwarg != nil {
		if sep {
			b.WriteString(", ")
		}
		b.WriteString("kwarg=")
		adArg(b, a.Kwarg)
		sep = true
	}
	if len(a.Defaults) > 0 {
		if sep {
			b.WriteString(", ")
		}
		b.WriteString("defaults=[")
		adExprList(b, a.Defaults, "Load")
		b.WriteByte(']')
	}
	b.WriteByte(')')
}

// adArg writes a single arg(...) node.
func adArg(b *strings.Builder, a *Arg) {
	b.WriteString("arg(arg=")
	b.WriteString(pyRepr(a.Name))
	if a.Annotation != nil {
		b.WriteString(", annotation=")
		adExpr(b, a.Annotation, "Load")
	}
	b.WriteByte(')')
}

// adComps writes the generators list contents (without outer brackets).
func adComps(b *strings.Builder, gens []*Comprehension) {
	for i, g := range gens {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("comprehension(target=")
		adExpr(b, g.Target, "Store")
		b.WriteString(", iter=")
		adExpr(b, g.Iter, "Load")
		if len(g.Ifs) > 0 {
			b.WriteString(", ifs=[")
			adExprList(b, g.Ifs, "Load")
			b.WriteByte(']')
		}
		if g.IsAsync {
			b.WriteString(", is_async=1)")
		} else {
			b.WriteString(", is_async=0)")
		}
	}
}

// adWithItems writes the withitem list contents (without outer brackets).
func adWithItems(b *strings.Builder, items []*WithItem) {
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("withitem(context_expr=")
		adExpr(b, it.ContextExpr, "Load")
		if it.OptionalVars != nil {
			b.WriteString(", optional_vars=")
			adExpr(b, it.OptionalVars, "Store")
		}
		b.WriteByte(')')
	}
}

// adExceptHandler writes an ExceptHandler(...) node.
func adExceptHandler(b *strings.Builder, h *ExceptHandler) {
	b.WriteString("ExceptHandler(")
	sep := false
	if h.Type != nil {
		b.WriteString("type=")
		adExpr(b, h.Type, "Load")
		sep = true
	}
	if h.Name != "" {
		if sep {
			b.WriteString(", ")
		}
		b.WriteString("name=")
		b.WriteString(pyRepr(h.Name))
		sep = true
	}
	if sep {
		b.WriteString(", ")
	}
	b.WriteString("body=")
	adStmtList(b, h.Body)
	b.WriteByte(')')
}

// adAlias writes an alias(...) node.
func adAlias(b *strings.Builder, a *Alias) {
	b.WriteString("alias(name=")
	b.WriteString(pyRepr(a.Name))
	if a.Asname != "" {
		b.WriteString(", asname=")
		b.WriteString(pyRepr(a.Asname))
	}
	b.WriteByte(')')
}

// adKeyword writes a keyword(...) node.
func adKeyword(b *strings.Builder, kw *Keyword) {
	b.WriteString("keyword(")
	if kw.Arg != "" {
		b.WriteString("arg=")
		b.WriteString(pyRepr(kw.Arg))
		b.WriteString(", ")
	}
	b.WriteString("value=")
	adExpr(b, kw.Value, "Load")
	b.WriteByte(')')
}

// adMatchCase writes a match_case(...) node.
func adMatchCase(b *strings.Builder, c *MatchCase) {
	b.WriteString("match_case(pattern=")
	adPattern(b, c.Pattern)
	if c.Guard != nil {
		b.WriteString(", guard=")
		adExpr(b, c.Guard, "Load")
	}
	b.WriteString(", body=")
	adStmtList(b, c.Body)
	b.WriteByte(')')
}

func adPattern(b *strings.Builder, p Pattern) {
	if p == nil {
		b.WriteString("None")
		return
	}
	switch n := p.(type) {
	case *MatchValue:
		b.WriteString("MatchValue(value=")
		adExpr(b, n.Value, "Load")
		b.WriteByte(')')

	case *MatchSingleton:
		b.WriteString("MatchSingleton(value=")
		switch v := n.Value.(type) {
		case nil:
			b.WriteString("None")
		case bool:
			if v {
				b.WriteString("True")
			} else {
				b.WriteString("False")
			}
		default:
			fmt.Fprintf(b, "%v", n.Value)
		}
		b.WriteByte(')')

	case *MatchSequence:
		if len(n.Patterns) == 0 {
			b.WriteString("MatchSequence()")
		} else {
			b.WriteString("MatchSequence(patterns=[")
			for i, q := range n.Patterns {
				if i > 0 {
					b.WriteString(", ")
				}
				adPattern(b, q)
			}
			b.WriteString("])")
		}

	case *MatchMapping:
		if len(n.Keys) == 0 && n.Rest == "" {
			b.WriteString("MatchMapping()")
		} else {
			b.WriteString("MatchMapping(")
			if len(n.Keys) > 0 {
				b.WriteString("keys=[")
				for i, k := range n.Keys {
					if i > 0 {
						b.WriteString(", ")
					}
					adExpr(b, k, "Load")
				}
				b.WriteString("], patterns=[")
				for i, q := range n.Patterns {
					if i > 0 {
						b.WriteString(", ")
					}
					adPattern(b, q)
				}
				b.WriteByte(']')
			}
			if n.Rest != "" {
				if len(n.Keys) > 0 {
					b.WriteString(", ")
				}
				b.WriteString("rest=")
				b.WriteString(pyRepr(n.Rest))
			}
			b.WriteByte(')')
		}

	case *MatchClass:
		b.WriteString("MatchClass(cls=")
		adExpr(b, n.Cls, "Load")
		if len(n.Patterns) > 0 {
			b.WriteString(", patterns=[")
			for i, q := range n.Patterns {
				if i > 0 {
					b.WriteString(", ")
				}
				adPattern(b, q)
			}
			b.WriteByte(']')
		}
		if len(n.KwdAttrs) > 0 {
			b.WriteString(", kwd_attrs=[")
			for i, a := range n.KwdAttrs {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(pyRepr(a))
			}
			b.WriteString("], kwd_patterns=[")
			for i, q := range n.KwdPatterns {
				if i > 0 {
					b.WriteString(", ")
				}
				adPattern(b, q)
			}
			b.WriteByte(']')
		}
		b.WriteByte(')')

	case *MatchStar:
		if n.Name == "" {
			b.WriteString("MatchStar()")
		} else {
			b.WriteString("MatchStar(name=")
			b.WriteString(pyRepr(n.Name))
			b.WriteByte(')')
		}

	case *MatchAs:
		if n.Pattern == nil && n.Name == "" {
			b.WriteString("MatchAs()")
			return
		}
		b.WriteString("MatchAs(")
		sep := false
		if n.Pattern != nil {
			b.WriteString("pattern=")
			adPattern(b, n.Pattern)
			sep = true
		}
		if n.Name != "" {
			if sep {
				b.WriteString(", ")
			}
			b.WriteString("name=")
			b.WriteString(pyRepr(n.Name))
		}
		b.WriteByte(')')

	case *MatchOr:
		b.WriteString("MatchOr(patterns=[")
		for i, q := range n.Patterns {
			if i > 0 {
				b.WriteString(", ")
			}
			adPattern(b, q)
		}
		b.WriteString("])")

	default:
		fmt.Fprintf(b, "<unknown pattern %T>", p)
	}
}

// adTypeParams writes type param nodes (PEP 695).
func adTypeParams(b *strings.Builder, ps []TypeParam) {
	for i, p := range ps {
		if i > 0 {
			b.WriteString(", ")
		}
		switch n := p.(type) {
		case *TypeVar:
			b.WriteString("TypeVar(name=")
			b.WriteString(pyRepr(n.Name))
			if n.Bound != nil {
				b.WriteString(", bound=")
				adExpr(b, n.Bound, "Load")
			}
			if n.DefaultValue != nil {
				b.WriteString(", default_value=")
				adExpr(b, n.DefaultValue, "Load")
			}
			b.WriteByte(')')
		case *TypeVarTuple:
			b.WriteString("TypeVarTuple(name=")
			b.WriteString(pyRepr(n.Name))
			if n.DefaultValue != nil {
				b.WriteString(", default_value=")
				adExpr(b, n.DefaultValue, "Load")
			}
			b.WriteByte(')')
		case *ParamSpec:
			b.WriteString("ParamSpec(name=")
			b.WriteString(pyRepr(n.Name))
			if n.DefaultValue != nil {
				b.WriteString(", default_value=")
				adExpr(b, n.DefaultValue, "Load")
			}
			b.WriteByte(')')
		default:
			fmt.Fprintf(b, "<unknown type_param %T>", p)
		}
	}
}
