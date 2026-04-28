package parser

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// dumper holds the output buffer and version-specific flags for ASTDump.
type dumper struct {
	b         strings.Builder
	pyMinor   int
	showEmpty bool // pyMinor <= 12: print empty/None optional fields
	py38      bool // pyMinor <= 8: 3.8-specific fields
}

func newDumper(pyMinor int) *dumper {
	return &dumper{
		pyMinor:   pyMinor,
		showEmpty: pyMinor <= 12,
		py38:      pyMinor <= 8,
	}
}

// ASTDump returns a single-line CPython ast.dump-style rendering of m.
// pyMinor selects the Python 3.x minor version (8–14) for version-aware output.
func ASTDump(m *Module, pyMinor int) string {
	d := newDumper(pyMinor)
	if len(m.Body) > 0 || d.showEmpty {
		d.b.WriteString("Module(body=[")
		for i, s := range m.Body {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adStmt(s)
		}
		d.b.WriteByte(']')
		if d.showEmpty {
			d.b.WriteString(", type_ignores=[]")
		}
		d.b.WriteByte(')')
	} else {
		d.b.WriteString("Module()")
	}
	return d.b.String()
}

// pythonIsPrint wraps unicode.IsPrint with version-gated overrides. Go ships
// Unicode 15.0 tables. Three codepoints became printable (So) in Unicode 15.1
// (Python 3.13) and five more in Unicode 16.0 (Python 3.14). Gate each group
// to keep dumps byte-identical across all supported minor versions.
func pythonIsPrint(r rune, pyMinor int) bool {
	if pyMinor >= 13 {
		switch r {
		case 0x2FFC, // IDEOGRAPHIC DESCRIPTION CHARACTER SURROUND FROM RIGHT -> So (Unicode 15.1)
			0x2FFF, // IDEOGRAPHIC DESCRIPTION CHARACTER ROTATION -> So (Unicode 15.1)
			0x31EF: // IDEOGRAPHIC DESCRIPTION CHARACTER SUBTRACTION -> So (Unicode 15.1)
			return true
		}
	}
	if pyMinor >= 14 {
		switch r {
		case 0x1B4F, // BALINESE INVERTED CARIK PAREREN -> Po (Unicode 16.0)
			0x1B7F, // BALINESE PANTI BAWAK -> Po (Unicode 16.0)
			0x1C89, // CYRILLIC CAPITAL LETTER TJE -> Lu (Unicode 16.0)
			0x2427, // SYMBOL FOR DELETE SQUARE CHECKER BOARD FORM -> So (Unicode 16.0)
			0x31E4: // CJK STROKE HXG -> So (Unicode 16.0)
			return true
		}
	}
	return unicode.IsPrint(r)
}

// pyRepr returns a Python repr-style quoted string (single-quote preferred).
// It handles WTF-8-encoded lone surrogates (U+D800–U+DFFF) by emitting \uXXXX,
// matching CPython's repr() output for strings containing lone surrogates.
func (d *dumper) pyRepr(s string) string {
	hasSingle := strings.ContainsRune(s, '\'')
	hasDouble := strings.ContainsRune(s, '"')
	useDouble := hasSingle && !hasDouble

	var b strings.Builder
	if useDouble {
		b.WriteByte('"')
	} else {
		b.WriteByte('\'')
	}
	for i := 0; i < len(s); {
		// Detect WTF-8 lone surrogate: ED [A0-BF] [80-BF]
		if i+3 <= len(s) && s[i] == 0xED && s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2] >= 0x80 && s[i+2] <= 0xBF {
			cp := rune(s[i]&0x0F)<<12 | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F)
			fmt.Fprintf(&b, `\u%04x`, cp)
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
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
		case r > 0x7f && !pythonIsPrint(r, d.pyMinor):
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

// pyComplexImag formats the imaginary part of a complex literal to match
// Python's repr(). It uses pyFloat but strips the trailing ".0" that Python
// omits for integer-valued imaginary parts (e.g. 123456789j not 123456789.0j).
func pyComplexImag(f float64) string {
	s := pyFloat(f)
	if strings.HasSuffix(s, ".0") {
		s = s[:len(s)-2]
	}
	return s
}

// adConstValue writes the Python repr of a Constant node's value.
// It is a method because it needs d.py38 for the kind=None field.
func (d *dumper) adConstValue(kind string, v any) {
	switch kind {
	case "None":
		d.b.WriteString("None")
	case "True":
		d.b.WriteString("True")
	case "False":
		d.b.WriteString("False")
	case "Ellipsis":
		d.b.WriteString("Ellipsis")
	case "str", "u":
		if s, ok := v.(string); ok {
			d.b.WriteString(d.pyRepr(s))
		}
	case "bytes":
		switch bv := v.(type) {
		case []byte:
			d.b.WriteString(pyBytesRepr(bv))
		case string:
			d.b.WriteString(pyBytesRepr([]byte(bv)))
		}
	case "float":
		switch fv := v.(type) {
		case float64:
			d.b.WriteString(pyFloat(fv))
		case float32:
			d.b.WriteString(pyFloat(float64(fv)))
		}
	case "complex":
		// Parser2 stores complex literals as a string like "1j" or "2.5j".
		if s, ok := v.(string); ok {
			s = strings.TrimRight(s, "jJ")
			f, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
			// ErrRange means overflow (+Inf) or underflow (0.0) — both valid.
			if err == nil || errors.Is(err, strconv.ErrRange) {
				d.b.WriteString(pyComplexImag(f))
			} else {
				d.b.WriteString(s)
			}
			d.b.WriteByte('j')
		} else if c, ok := v.(complex128); ok {
			d.b.WriteString(pyComplexImag(imag(c)))
			d.b.WriteByte('j')
		}
	default:
		// *big.Int prints as decimal via its String() method, which matches
		// CPython's repr() for large integer constants.
		if bi, ok := v.(*big.Int); ok {
			d.b.WriteString(bi.String())
		} else {
			fmt.Fprintf(&d.b, "%v", v)
		}
	}
}

// adOp writes an operator name with parentheses, e.g. "Add()".
func (d *dumper) adOp(op string) {
	d.b.WriteString(op)
	d.b.WriteString("()")
}

// adCtx writes ", ctx=Load()" etc.
func (d *dumper) adCtx(ctx string) {
	d.b.WriteString(", ctx=")
	d.b.WriteString(ctx)
	d.b.WriteString("()")
}

func (d *dumper) adStmtList(ss []Stmt) {
	d.b.WriteByte('[')
	for i, s := range ss {
		if i > 0 {
			d.b.WriteString(", ")
		}
		d.adStmt(s)
	}
	d.b.WriteByte(']')
}

func (d *dumper) adExprList(es []Expr, ctx string) {
	for i, e := range es {
		if i > 0 {
			d.b.WriteString(", ")
		}
		d.adExpr(e, ctx)
	}
}

func (d *dumper) adStmt(s Stmt) {
	if s == nil {
		d.b.WriteString("None")
		return
	}
	switch n := s.(type) {
	case *ExprStmt:
		d.b.WriteString("Expr(value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteByte(')')

	case *Assign:
		d.b.WriteString("Assign(targets=[")
		for i, t := range n.Targets {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adExpr(t, "Store")
		}
		d.b.WriteString("], value=")
		d.adExpr(n.Value, "Load")
		if d.py38 {
			d.b.WriteString(", type_comment=None")
		}
		d.b.WriteByte(')')

	case *AugAssign:
		d.b.WriteString("AugAssign(target=")
		d.adExpr(n.Target, "Store")
		d.b.WriteString(", op=")
		d.adOp(n.Op)
		d.b.WriteString(", value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteByte(')')

	case *AnnAssign:
		d.b.WriteString("AnnAssign(target=")
		d.adExpr(n.Target, "Store")
		d.b.WriteString(", annotation=")
		d.adExpr(n.Annotation, "Load")
		if n.Value != nil {
			d.b.WriteString(", value=")
			d.adExpr(n.Value, "Load")
		} else if d.py38 {
			d.b.WriteString(", value=None")
		}
		if n.Simple {
			d.b.WriteString(", simple=1)")
		} else {
			d.b.WriteString(", simple=0)")
		}

	case *Return:
		if n.Value != nil {
			d.b.WriteString("Return(value=")
			d.adExpr(n.Value, "Load")
			d.b.WriteByte(')')
		} else if d.py38 {
			d.b.WriteString("Return(value=None)")
		} else {
			d.b.WriteString("Return()")
		}

	case *Delete:
		d.b.WriteString("Delete(targets=[")
		for i, t := range n.Targets {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adExpr(t, "Del")
		}
		d.b.WriteString("])")

	case *Pass:
		d.b.WriteString("Pass()")
	case *Break:
		d.b.WriteString("Break()")
	case *Continue:
		d.b.WriteString("Continue()")

	case *Raise:
		if n.Exc == nil {
			if d.py38 {
				d.b.WriteString("Raise(exc=None, cause=None)")
			} else {
				d.b.WriteString("Raise()")
			}
		} else {
			d.b.WriteString("Raise(exc=")
			d.adExpr(n.Exc, "Load")
			if n.Cause != nil {
				d.b.WriteString(", cause=")
				d.adExpr(n.Cause, "Load")
			} else if d.py38 {
				d.b.WriteString(", cause=None")
			}
			d.b.WriteByte(')')
		}

	case *Assert:
		d.b.WriteString("Assert(test=")
		d.adExpr(n.Test, "Load")
		if n.Msg != nil {
			d.b.WriteString(", msg=")
			d.adExpr(n.Msg, "Load")
		} else if d.py38 {
			d.b.WriteString(", msg=None")
		}
		d.b.WriteByte(')')

	case *Global:
		d.b.WriteString("Global(names=[")
		for i, name := range n.Names {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.b.WriteString(d.pyRepr(name))
		}
		d.b.WriteString("])")

	case *Nonlocal:
		d.b.WriteString("Nonlocal(names=[")
		for i, name := range n.Names {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.b.WriteString(d.pyRepr(name))
		}
		d.b.WriteString("])")

	case *Import:
		d.b.WriteString("Import(names=[")
		for i, a := range n.Names {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adAlias(a)
		}
		d.b.WriteString("])")

	case *ImportFrom:
		d.b.WriteString("ImportFrom(")
		if n.Module != "" {
			d.b.WriteString("module=")
			d.b.WriteString(d.pyRepr(n.Module))
			d.b.WriteString(", ")
		} else if d.py38 {
			d.b.WriteString("module=None, ")
		}
		d.b.WriteString("names=[")
		for i, a := range n.Names {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adAlias(a)
		}
		fmt.Fprintf(&d.b, "], level=%d)", n.Level)

	case *If:
		d.b.WriteString("If(test=")
		d.adExpr(n.Test, "Load")
		d.b.WriteString(", body=")
		d.adStmtList(n.Body)
		if len(n.Orelse) > 0 || d.showEmpty {
			d.b.WriteString(", orelse=")
			d.adStmtList(n.Orelse)
		}
		d.b.WriteByte(')')

	case *While:
		d.b.WriteString("While(test=")
		d.adExpr(n.Test, "Load")
		d.b.WriteString(", body=")
		d.adStmtList(n.Body)
		if len(n.Orelse) > 0 || d.showEmpty {
			d.b.WriteString(", orelse=")
			d.adStmtList(n.Orelse)
		}
		d.b.WriteByte(')')

	case *For:
		d.b.WriteString("For(target=")
		d.adExpr(n.Target, "Store")
		d.b.WriteString(", iter=")
		d.adExpr(n.Iter, "Load")
		d.b.WriteString(", body=")
		d.adStmtList(n.Body)
		if len(n.Orelse) > 0 || d.showEmpty {
			d.b.WriteString(", orelse=")
			d.adStmtList(n.Orelse)
		}
		if d.py38 {
			d.b.WriteString(", type_comment=None")
		}
		d.b.WriteByte(')')

	case *AsyncFor:
		d.b.WriteString("AsyncFor(target=")
		d.adExpr(n.Target, "Store")
		d.b.WriteString(", iter=")
		d.adExpr(n.Iter, "Load")
		d.b.WriteString(", body=")
		d.adStmtList(n.Body)
		if len(n.Orelse) > 0 || d.showEmpty {
			d.b.WriteString(", orelse=")
			d.adStmtList(n.Orelse)
		}
		if d.py38 {
			d.b.WriteString(", type_comment=None")
		}
		d.b.WriteByte(')')

	case *With:
		d.b.WriteString("With(items=[")
		d.adWithItems(n.Items)
		d.b.WriteString("], body=")
		d.adStmtList(n.Body)
		if d.py38 {
			d.b.WriteString(", type_comment=None")
		}
		d.b.WriteByte(')')

	case *AsyncWith:
		d.b.WriteString("AsyncWith(items=[")
		d.adWithItems(n.Items)
		d.b.WriteString("], body=")
		d.adStmtList(n.Body)
		if d.py38 {
			d.b.WriteString(", type_comment=None")
		}
		d.b.WriteByte(')')

	case *Try:
		d.b.WriteString("Try(body=")
		d.adStmtList(n.Body)
		if len(n.Handlers) > 0 || d.showEmpty {
			d.b.WriteString(", handlers=[")
			for i, h := range n.Handlers {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.adExceptHandler(h)
			}
			d.b.WriteByte(']')
		}
		if len(n.Orelse) > 0 || d.showEmpty {
			d.b.WriteString(", orelse=")
			d.adStmtList(n.Orelse)
		}
		if len(n.Finalbody) > 0 || d.showEmpty {
			d.b.WriteString(", finalbody=")
			d.adStmtList(n.Finalbody)
		}
		d.b.WriteByte(')')

	case *TryStar:
		d.b.WriteString("TryStar(body=")
		d.adStmtList(n.Body)
		if len(n.Handlers) > 0 || d.showEmpty {
			d.b.WriteString(", handlers=[")
			for i, h := range n.Handlers {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.adExceptHandler(h)
			}
			d.b.WriteByte(']')
		}
		if len(n.Orelse) > 0 || d.showEmpty {
			d.b.WriteString(", orelse=")
			d.adStmtList(n.Orelse)
		}
		if len(n.Finalbody) > 0 || d.showEmpty {
			d.b.WriteString(", finalbody=")
			d.adStmtList(n.Finalbody)
		}
		d.b.WriteByte(')')

	case *FunctionDef:
		d.b.WriteString("FunctionDef(name=")
		d.b.WriteString(d.pyRepr(n.Name))
		d.b.WriteString(", args=")
		d.adArgs(n.Args)
		d.b.WriteString(", body=")
		d.adStmtList(n.Body)
		if len(n.DecoratorList) > 0 || d.showEmpty {
			d.b.WriteString(", decorator_list=[")
			d.adExprList(n.DecoratorList, "Load")
			d.b.WriteByte(']')
		}
		if n.Returns != nil {
			d.b.WriteString(", returns=")
			d.adExpr(n.Returns, "Load")
		} else if d.py38 {
			d.b.WriteString(", returns=None")
		}
		if d.pyMinor >= 12 && (len(n.TypeParams) > 0 || d.showEmpty) {
			d.b.WriteString(", type_params=[")
			d.adTypeParams(n.TypeParams)
			d.b.WriteByte(']')
		}
		if d.py38 {
			d.b.WriteString(", type_comment=None")
		}
		d.b.WriteByte(')')

	case *AsyncFunctionDef:
		d.b.WriteString("AsyncFunctionDef(name=")
		d.b.WriteString(d.pyRepr(n.Name))
		d.b.WriteString(", args=")
		d.adArgs(n.Args)
		d.b.WriteString(", body=")
		d.adStmtList(n.Body)
		if len(n.DecoratorList) > 0 || d.showEmpty {
			d.b.WriteString(", decorator_list=[")
			d.adExprList(n.DecoratorList, "Load")
			d.b.WriteByte(']')
		}
		if n.Returns != nil {
			d.b.WriteString(", returns=")
			d.adExpr(n.Returns, "Load")
		} else if d.py38 {
			d.b.WriteString(", returns=None")
		}
		if d.pyMinor >= 12 && (len(n.TypeParams) > 0 || d.showEmpty) {
			d.b.WriteString(", type_params=[")
			d.adTypeParams(n.TypeParams)
			d.b.WriteByte(']')
		}
		if d.py38 {
			d.b.WriteString(", type_comment=None")
		}
		d.b.WriteByte(')')

	case *ClassDef:
		d.b.WriteString("ClassDef(name=")
		d.b.WriteString(d.pyRepr(n.Name))
		if len(n.Bases) > 0 || d.showEmpty {
			d.b.WriteString(", bases=[")
			d.adExprList(n.Bases, "Load")
			d.b.WriteByte(']')
		}
		if len(n.Keywords) > 0 || d.showEmpty {
			d.b.WriteString(", keywords=[")
			for i, kw := range n.Keywords {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.adKeyword(kw)
			}
			d.b.WriteByte(']')
		}
		d.b.WriteString(", body=")
		d.adStmtList(n.Body)
		if len(n.DecoratorList) > 0 || d.showEmpty {
			d.b.WriteString(", decorator_list=[")
			d.adExprList(n.DecoratorList, "Load")
			d.b.WriteByte(']')
		}
		if d.pyMinor >= 12 && (len(n.TypeParams) > 0 || d.showEmpty) {
			d.b.WriteString(", type_params=[")
			d.adTypeParams(n.TypeParams)
			d.b.WriteByte(']')
		}
		d.b.WriteByte(')')

	case *TypeAlias:
		d.b.WriteString("TypeAlias(name=")
		d.adExpr(n.Name, "Store")
		// TypeAlias is 3.12+; always show type_params (empty or not) when showEmpty
		if len(n.TypeParams) > 0 || d.showEmpty {
			d.b.WriteString(", type_params=[")
			d.adTypeParams(n.TypeParams)
			d.b.WriteByte(']')
		}
		d.b.WriteString(", value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteByte(')')

	case *Match:
		d.b.WriteString("Match(subject=")
		d.adExpr(n.Subject, "Load")
		d.b.WriteString(", cases=[")
		for i, c := range n.Cases {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adMatchCase(c)
		}
		d.b.WriteString("])")

	default:
		fmt.Fprintf(&d.b, "<unknown stmt %T>", s)
	}
}

func (d *dumper) adExpr(e Expr, ctx string) {
	if e == nil {
		d.b.WriteString("None")
		return
	}
	switch n := e.(type) {
	case *Constant:
		d.b.WriteString("Constant(value=")
		d.adConstValue(n.Kind, n.Value)
		if n.Kind == "u" {
			d.b.WriteString(", kind='u'")
		} else if d.py38 {
			// In 3.8, kind=None is printed for all non-u-string constants
			d.b.WriteString(", kind=None")
		}
		d.b.WriteByte(')')

	case *Name:
		d.b.WriteString("Name(id=")
		d.b.WriteString(d.pyRepr(n.Id))
		d.adCtx(ctx)
		d.b.WriteByte(')')

	case *Attribute:
		d.b.WriteString("Attribute(value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteString(", attr=")
		d.b.WriteString(d.pyRepr(n.Attr))
		d.adCtx(ctx)
		d.b.WriteByte(')')

	case *Subscript:
		d.b.WriteString("Subscript(value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteString(", slice=")
		if d.py38 {
			if tup, isTuple := n.Slice.(*Tuple); isTuple {
				// Python 3.8: ExtSlice only when at least one element is a Slice;
				// otherwise wrap the whole tuple in Index(value=Tuple(...)).
				hasSlice := false
				for _, elt := range tup.Elts {
					if _, ok := elt.(*Slice); ok {
						hasSlice = true
						break
					}
				}
				if hasSlice {
					d.b.WriteString("ExtSlice(dims=[")
					for i, elt := range tup.Elts {
						if i > 0 {
							d.b.WriteString(", ")
						}
						if _, eltIsSlice := elt.(*Slice); eltIsSlice {
							d.adExpr(elt, "Load")
						} else {
							d.b.WriteString("Index(value=")
							d.adExpr(elt, "Load")
							d.b.WriteByte(')')
						}
					}
					d.b.WriteString("])")
				} else {
					// No Slice in tuple — wrap whole tuple in Index
					d.b.WriteString("Index(value=")
					d.adExpr(n.Slice, "Load")
					d.b.WriteByte(')')
				}
			} else if _, isSlice := n.Slice.(*Slice); isSlice {
				d.adExpr(n.Slice, "Load")
			} else {
				d.b.WriteString("Index(value=")
				d.adExpr(n.Slice, "Load")
				d.b.WriteByte(')')
			}
		} else {
			d.adExpr(n.Slice, "Load")
		}
		d.adCtx(ctx)
		d.b.WriteByte(')')

	case *Starred:
		d.b.WriteString("Starred(value=")
		d.adExpr(n.Value, ctx)
		d.adCtx(ctx)
		d.b.WriteByte(')')

	case *List:
		d.b.WriteString("List(")
		if len(n.Elts) > 0 {
			d.b.WriteString("elts=[")
			d.adExprList(n.Elts, ctx)
			d.b.WriteString("], ctx=")
			d.b.WriteString(ctx)
			d.b.WriteString("())")
		} else if d.showEmpty {
			d.b.WriteString("elts=[], ctx=")
			d.b.WriteString(ctx)
			d.b.WriteString("())")
		} else {
			d.b.WriteString("ctx=")
			d.b.WriteString(ctx)
			d.b.WriteString("())")
		}

	case *Tuple:
		d.b.WriteString("Tuple(")
		if len(n.Elts) > 0 {
			d.b.WriteString("elts=[")
			d.adExprList(n.Elts, ctx)
			d.b.WriteString("], ctx=")
			d.b.WriteString(ctx)
			d.b.WriteString("())")
		} else if d.showEmpty {
			d.b.WriteString("elts=[], ctx=")
			d.b.WriteString(ctx)
			d.b.WriteString("())")
		} else {
			d.b.WriteString("ctx=")
			d.b.WriteString(ctx)
			d.b.WriteString("())")
		}

	case *Set:
		d.b.WriteString("Set(elts=[")
		d.adExprList(n.Elts, "Load")
		d.b.WriteString("])")

	case *Dict:
		if !d.showEmpty && len(n.Keys) == 0 {
			d.b.WriteString("Dict()")
			break
		}
		d.b.WriteString("Dict(keys=[")
		for i, k := range n.Keys {
			if i > 0 {
				d.b.WriteString(", ")
			}
			if k == nil {
				d.b.WriteString("None")
			} else {
				d.adExpr(k, "Load")
			}
		}
		d.b.WriteString("], values=[")
		d.adExprList(n.Values, "Load")
		d.b.WriteString("])")

	case *BinOp:
		d.b.WriteString("BinOp(left=")
		d.adExpr(n.Left, "Load")
		d.b.WriteString(", op=")
		d.adOp(n.Op)
		d.b.WriteString(", right=")
		d.adExpr(n.Right, "Load")
		d.b.WriteByte(')')

	case *UnaryOp:
		d.b.WriteString("UnaryOp(op=")
		d.adOp(n.Op)
		d.b.WriteString(", operand=")
		d.adExpr(n.Operand, "Load")
		d.b.WriteByte(')')

	case *BoolOp:
		d.b.WriteString("BoolOp(op=")
		d.adOp(n.Op)
		d.b.WriteString(", values=[")
		d.adExprList(n.Values, "Load")
		d.b.WriteString("])")

	case *Compare:
		d.b.WriteString("Compare(left=")
		d.adExpr(n.Left, "Load")
		d.b.WriteString(", ops=[")
		for i, op := range n.Ops {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adOp(op)
		}
		d.b.WriteString("], comparators=[")
		d.adExprList(n.Comparators, "Load")
		d.b.WriteString("])")

	case *Call:
		d.b.WriteString("Call(func=")
		d.adExpr(n.Func, "Load")
		if len(n.Args) > 0 || d.showEmpty {
			d.b.WriteString(", args=[")
			d.adExprList(n.Args, "Load")
			d.b.WriteByte(']')
		}
		if len(n.Keywords) > 0 || d.showEmpty {
			d.b.WriteString(", keywords=[")
			for i, kw := range n.Keywords {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.adKeyword(kw)
			}
			d.b.WriteByte(']')
		}
		d.b.WriteByte(')')

	case *IfExp:
		d.b.WriteString("IfExp(test=")
		d.adExpr(n.Test, "Load")
		d.b.WriteString(", body=")
		d.adExpr(n.Body, "Load")
		d.b.WriteString(", orelse=")
		d.adExpr(n.OrElse, "Load")
		d.b.WriteByte(')')

	case *Lambda:
		d.b.WriteString("Lambda(args=")
		d.adArgs(n.Args)
		d.b.WriteString(", body=")
		d.adExpr(n.Body, "Load")
		d.b.WriteByte(')')

	case *NamedExpr:
		d.b.WriteString("NamedExpr(target=")
		d.adExpr(n.Target, "Store")
		d.b.WriteString(", value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteByte(')')

	case *Await:
		d.b.WriteString("Await(value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteByte(')')

	case *Yield:
		if n.Value != nil {
			d.b.WriteString("Yield(value=")
			d.adExpr(n.Value, "Load")
			d.b.WriteByte(')')
		} else if d.py38 {
			d.b.WriteString("Yield(value=None)")
		} else {
			d.b.WriteString("Yield()")
		}

	case *YieldFrom:
		d.b.WriteString("YieldFrom(value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteByte(')')

	case *JoinedStr:
		if len(n.Values) == 0 {
			if d.showEmpty {
				d.b.WriteString("JoinedStr(values=[])")
			} else {
				d.b.WriteString("JoinedStr()")
			}
		} else {
			d.b.WriteString("JoinedStr(values=[")
			for i, v := range n.Values {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.adExpr(v, "Load")
			}
			d.b.WriteString("])")
		}

	case *FormattedValue:
		d.b.WriteString("FormattedValue(value=")
		d.adExpr(n.Value, "Load")
		fmt.Fprintf(&d.b, ", conversion=%d", n.Conversion)
		if n.FormatSpec != nil {
			d.b.WriteString(", format_spec=")
			if js, ok := n.FormatSpec.(*JoinedStr); ok {
				d.adFormatSpec(js)
			} else {
				d.adExpr(n.FormatSpec, "Load")
			}
		} else if d.py38 {
			d.b.WriteString(", format_spec=None")
		}
		d.b.WriteByte(')')

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
			d.b.WriteString("TemplateStr()")
		} else {
			d.b.WriteString("TemplateStr(values=[")
			for i, it := range flat {
				if i > 0 {
					d.b.WriteString(", ")
				}
				if it.str != nil {
					d.adExpr(it.str, "Load")
				} else {
					d.adExpr(it.interp, "Load")
				}
			}
			d.b.WriteString("])")
		}

	case *Interpolation:
		d.b.WriteString("Interpolation(value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteString(", str=")
		d.b.WriteString(d.pyRepr(n.Str))
		fmt.Fprintf(&d.b, ", conversion=%d", n.Conversion)
		if n.FormatSpec != nil {
			d.b.WriteString(", format_spec=")
			d.adExpr(n.FormatSpec, "Load")
		}
		d.b.WriteByte(')')

	case *Slice:
		if d.py38 {
			// In 3.8, always print all three fields, even when nil
			d.b.WriteString("Slice(lower=")
			if n.Lower != nil {
				d.adExpr(n.Lower, "Load")
			} else {
				d.b.WriteString("None")
			}
			d.b.WriteString(", upper=")
			if n.Upper != nil {
				d.adExpr(n.Upper, "Load")
			} else {
				d.b.WriteString("None")
			}
			d.b.WriteString(", step=")
			if n.Step != nil {
				d.adExpr(n.Step, "Load")
			} else {
				d.b.WriteString("None")
			}
			d.b.WriteByte(')')
		} else {
			first := true
			buf := &strings.Builder{}
			if n.Lower != nil {
				buf.WriteString("lower=")
				tmpD := &dumper{pyMinor: d.pyMinor, showEmpty: d.showEmpty, py38: d.py38}
				tmpD.adExpr(n.Lower, "Load")
				buf.WriteString(tmpD.b.String())
				first = false
			}
			if n.Upper != nil {
				if !first {
					buf.WriteString(", ")
				}
				buf.WriteString("upper=")
				tmpD := &dumper{pyMinor: d.pyMinor, showEmpty: d.showEmpty, py38: d.py38}
				tmpD.adExpr(n.Upper, "Load")
				buf.WriteString(tmpD.b.String())
				first = false
			}
			if n.Step != nil {
				if !first {
					buf.WriteString(", ")
				}
				buf.WriteString("step=")
				tmpD := &dumper{pyMinor: d.pyMinor, showEmpty: d.showEmpty, py38: d.py38}
				tmpD.adExpr(n.Step, "Load")
				buf.WriteString(tmpD.b.String())
			}
			d.b.WriteString("Slice(")
			d.b.WriteString(buf.String())
			d.b.WriteByte(')')
		}

	case *ListComp:
		d.b.WriteString("ListComp(elt=")
		d.adExpr(n.Elt, "Load")
		d.b.WriteString(", generators=[")
		d.adComps(n.Gens)
		d.b.WriteString("])")

	case *SetComp:
		d.b.WriteString("SetComp(elt=")
		d.adExpr(n.Elt, "Load")
		d.b.WriteString(", generators=[")
		d.adComps(n.Gens)
		d.b.WriteString("])")

	case *DictComp:
		d.b.WriteString("DictComp(key=")
		d.adExpr(n.Key, "Load")
		d.b.WriteString(", value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteString(", generators=[")
		d.adComps(n.Gens)
		d.b.WriteString("])")

	case *GeneratorExp:
		d.b.WriteString("GeneratorExp(elt=")
		d.adExpr(n.Elt, "Load")
		d.b.WriteString(", generators=[")
		d.adComps(n.Gens)
		d.b.WriteString("])")

	default:
		fmt.Fprintf(&d.b, "<unknown expr %T>", e)
	}
}

// adArgs writes an arguments(...) node.
func (d *dumper) adArgs(a *Arguments) {
	if a == nil {
		d.b.WriteString("arguments()")
		return
	}
	d.b.WriteString("arguments(")
	sep := false
	if len(a.PosOnly) > 0 || d.showEmpty {
		d.b.WriteString("posonlyargs=[")
		for i, p := range a.PosOnly {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adArg(p)
		}
		d.b.WriteByte(']')
		sep = true
	}
	if len(a.Args) > 0 || d.showEmpty {
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("args=[")
		for i, p := range a.Args {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adArg(p)
		}
		d.b.WriteByte(']')
		sep = true
	}
	if a.Vararg != nil {
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("vararg=")
		d.adArg(a.Vararg)
		sep = true
	} else if d.py38 {
		// Only 3.8 prints vararg=None
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("vararg=None")
		sep = true
	}
	if len(a.KwOnly) > 0 || d.showEmpty {
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("kwonlyargs=[")
		for i, p := range a.KwOnly {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adArg(p)
		}
		d.b.WriteByte(']')
		sep = true
	}
	if len(a.KwOnlyDef) > 0 || d.showEmpty {
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("kw_defaults=[")
		for i, def := range a.KwOnlyDef {
			if i > 0 {
				d.b.WriteString(", ")
			}
			if def == nil {
				d.b.WriteString("None")
			} else {
				d.adExpr(def, "Load")
			}
		}
		d.b.WriteByte(']')
		sep = true
	}
	if a.Kwarg != nil {
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("kwarg=")
		d.adArg(a.Kwarg)
		sep = true
	} else if d.py38 {
		// Only 3.8 prints kwarg=None
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("kwarg=None")
		sep = true
	}
	if len(a.Defaults) > 0 || d.showEmpty {
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("defaults=[")
		d.adExprList(a.Defaults, "Load")
		d.b.WriteByte(']')
	}
	d.b.WriteByte(')')
}

// adArg writes a single arg(...) node.
func (d *dumper) adArg(a *Arg) {
	d.b.WriteString("arg(arg=")
	d.b.WriteString(d.pyRepr(a.Name))
	if a.Annotation != nil {
		d.b.WriteString(", annotation=")
		d.adExpr(a.Annotation, "Load")
	} else if d.py38 {
		d.b.WriteString(", annotation=None")
	}
	if d.py38 {
		d.b.WriteString(", type_comment=None")
	}
	d.b.WriteByte(')')
}

// adComps writes the generators list contents (without outer brackets).
func (d *dumper) adComps(gens []*Comprehension) {
	for i, g := range gens {
		if i > 0 {
			d.b.WriteString(", ")
		}
		d.b.WriteString("comprehension(target=")
		d.adExpr(g.Target, "Store")
		d.b.WriteString(", iter=")
		d.adExpr(g.Iter, "Load")
		if len(g.Ifs) > 0 || d.showEmpty {
			d.b.WriteString(", ifs=[")
			d.adExprList(g.Ifs, "Load")
			d.b.WriteByte(']')
		}
		if g.IsAsync {
			d.b.WriteString(", is_async=1)")
		} else {
			d.b.WriteString(", is_async=0)")
		}
	}
}

// adWithItems writes the withitem list contents (without outer brackets).
func (d *dumper) adWithItems(items []*WithItem) {
	for i, it := range items {
		if i > 0 {
			d.b.WriteString(", ")
		}
		d.b.WriteString("withitem(context_expr=")
		d.adExpr(it.ContextExpr, "Load")
		if it.OptionalVars != nil {
			d.b.WriteString(", optional_vars=")
			d.adExpr(it.OptionalVars, "Store")
		} else if d.py38 {
			d.b.WriteString(", optional_vars=None")
		}
		d.b.WriteByte(')')
	}
}

// adExceptHandler writes an ExceptHandler(...) node.
func (d *dumper) adExceptHandler(h *ExceptHandler) {
	d.b.WriteString("ExceptHandler(")
	sep := false
	if h.Type != nil {
		d.b.WriteString("type=")
		d.adExpr(h.Type, "Load")
		sep = true
	} else if d.py38 {
		d.b.WriteString("type=None")
		sep = true
	}
	if h.Name != "" {
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("name=")
		d.b.WriteString(d.pyRepr(h.Name))
		sep = true
	} else if d.py38 {
		if sep {
			d.b.WriteString(", ")
		}
		d.b.WriteString("name=None")
		sep = true
	}
	if sep {
		d.b.WriteString(", ")
	}
	d.b.WriteString("body=")
	d.adStmtList(h.Body)
	d.b.WriteByte(')')
}

// adAlias writes an alias(...) node.
func (d *dumper) adAlias(a *Alias) {
	d.b.WriteString("alias(name=")
	d.b.WriteString(d.pyRepr(a.Name))
	if a.Asname != "" {
		d.b.WriteString(", asname=")
		d.b.WriteString(d.pyRepr(a.Asname))
	} else if d.py38 {
		d.b.WriteString(", asname=None")
	}
	d.b.WriteByte(')')
}

// adKeyword writes a keyword(...) node.
func (d *dumper) adKeyword(kw *Keyword) {
	d.b.WriteString("keyword(")
	if kw.Arg != "" {
		d.b.WriteString("arg=")
		d.b.WriteString(d.pyRepr(kw.Arg))
		d.b.WriteString(", ")
	} else if d.py38 {
		d.b.WriteString("arg=None, ")
	}
	d.b.WriteString("value=")
	d.adExpr(kw.Value, "Load")
	d.b.WriteByte(')')
}

// adMatchCase writes a match_case(...) node.
func (d *dumper) adMatchCase(c *MatchCase) {
	d.b.WriteString("match_case(pattern=")
	d.adPattern(c.Pattern)
	if c.Guard != nil {
		d.b.WriteString(", guard=")
		d.adExpr(c.Guard, "Load")
	}
	d.b.WriteString(", body=")
	d.adStmtList(c.Body)
	d.b.WriteByte(')')
}

func (d *dumper) adPattern(p Pattern) {
	if p == nil {
		d.b.WriteString("None")
		return
	}
	switch n := p.(type) {
	case *MatchValue:
		d.b.WriteString("MatchValue(value=")
		d.adExpr(n.Value, "Load")
		d.b.WriteByte(')')

	case *MatchSingleton:
		d.b.WriteString("MatchSingleton(value=")
		switch v := n.Value.(type) {
		case nil:
			d.b.WriteString("None")
		case bool:
			if v {
				d.b.WriteString("True")
			} else {
				d.b.WriteString("False")
			}
		default:
			fmt.Fprintf(&d.b, "%v", n.Value)
		}
		d.b.WriteByte(')')

	case *MatchSequence:
		if len(n.Patterns) == 0 {
			if d.showEmpty {
				d.b.WriteString("MatchSequence(patterns=[])")
			} else {
				d.b.WriteString("MatchSequence()")
			}
		} else {
			d.b.WriteString("MatchSequence(patterns=[")
			for i, q := range n.Patterns {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.adPattern(q)
			}
			d.b.WriteString("])")
		}

	case *MatchMapping:
		if len(n.Keys) == 0 && n.Rest == "" {
			if d.showEmpty {
				d.b.WriteString("MatchMapping(keys=[], patterns=[])")
			} else {
				d.b.WriteString("MatchMapping()")
			}
		} else {
			d.b.WriteString("MatchMapping(")
			if len(n.Keys) > 0 {
				d.b.WriteString("keys=[")
				for i, k := range n.Keys {
					if i > 0 {
						d.b.WriteString(", ")
					}
					d.adExpr(k, "Load")
				}
				d.b.WriteString("], patterns=[")
				for i, q := range n.Patterns {
					if i > 0 {
						d.b.WriteString(", ")
					}
					d.adPattern(q)
				}
				d.b.WriteByte(']')
			}
			if n.Rest != "" {
				if len(n.Keys) > 0 {
					d.b.WriteString(", ")
				}
				d.b.WriteString("rest=")
				d.b.WriteString(d.pyRepr(n.Rest))
			}
			d.b.WriteByte(')')
		}

	case *MatchClass:
		d.b.WriteString("MatchClass(cls=")
		d.adExpr(n.Cls, "Load")
		if len(n.Patterns) > 0 || d.showEmpty {
			d.b.WriteString(", patterns=[")
			for i, q := range n.Patterns {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.adPattern(q)
			}
			d.b.WriteByte(']')
		}
		if len(n.KwdAttrs) > 0 || d.showEmpty {
			d.b.WriteString(", kwd_attrs=[")
			for i, a := range n.KwdAttrs {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.b.WriteString(d.pyRepr(a))
			}
			d.b.WriteString("], kwd_patterns=[")
			for i, q := range n.KwdPatterns {
				if i > 0 {
					d.b.WriteString(", ")
				}
				d.adPattern(q)
			}
			d.b.WriteByte(']')
		}
		d.b.WriteByte(')')

	case *MatchStar:
		if n.Name == "" {
			d.b.WriteString("MatchStar()")
		} else {
			d.b.WriteString("MatchStar(name=")
			d.b.WriteString(d.pyRepr(n.Name))
			d.b.WriteByte(')')
		}

	case *MatchAs:
		if n.Pattern == nil && n.Name == "" {
			d.b.WriteString("MatchAs()")
			return
		}
		d.b.WriteString("MatchAs(")
		sep := false
		if n.Pattern != nil {
			d.b.WriteString("pattern=")
			d.adPattern(n.Pattern)
			sep = true
		}
		if n.Name != "" {
			if sep {
				d.b.WriteString(", ")
			}
			d.b.WriteString("name=")
			d.b.WriteString(d.pyRepr(n.Name))
		}
		d.b.WriteByte(')')

	case *MatchOr:
		d.b.WriteString("MatchOr(patterns=[")
		for i, q := range n.Patterns {
			if i > 0 {
				d.b.WriteString(", ")
			}
			d.adPattern(q)
		}
		d.b.WriteString("])")

	default:
		fmt.Fprintf(&d.b, "<unknown pattern %T>", p)
	}
}

// adTypeParams writes type param nodes (PEP 695).
func (d *dumper) adTypeParams(ps []TypeParam) {
	for i, p := range ps {
		if i > 0 {
			d.b.WriteString(", ")
		}
		switch n := p.(type) {
		case *TypeVar:
			d.b.WriteString("TypeVar(name=")
			d.b.WriteString(d.pyRepr(n.Name))
			if n.Bound != nil {
				d.b.WriteString(", bound=")
				d.adExpr(n.Bound, "Load")
			}
			if n.DefaultValue != nil {
				d.b.WriteString(", default_value=")
				d.adExpr(n.DefaultValue, "Load")
			}
			d.b.WriteByte(')')
		case *TypeVarTuple:
			d.b.WriteString("TypeVarTuple(name=")
			d.b.WriteString(d.pyRepr(n.Name))
			if n.DefaultValue != nil {
				d.b.WriteString(", default_value=")
				d.adExpr(n.DefaultValue, "Load")
			}
			d.b.WriteByte(')')
		case *ParamSpec:
			d.b.WriteString("ParamSpec(name=")
			d.b.WriteString(d.pyRepr(n.Name))
			if n.DefaultValue != nil {
				d.b.WriteString(", default_value=")
				d.adExpr(n.DefaultValue, "Load")
			}
			d.b.WriteByte(')')
		default:
			fmt.Fprintf(&d.b, "<unknown type_param %T>", p)
		}
	}
}

// adFormatSpec dumps a format_spec JoinedStr, applying the Python 3.12 rule:
// when pyMinor <= 12 and the last value is a FormattedValue, append a trailing
// Constant(value='') to match CPython 3.12's AST representation.
func (d *dumper) adFormatSpec(spec *JoinedStr) {
	values := spec.Values
	if d.showEmpty && len(values) > 0 {
		if _, isFormattedValue := values[len(values)-1].(*FormattedValue); isFormattedValue {
			trailing := &Constant{P: spec.P, Kind: "str", Value: ""}
			values = append(values, trailing)
		}
	}
	if len(values) == 0 {
		if d.showEmpty {
			d.b.WriteString("JoinedStr(values=[])")
		} else {
			d.b.WriteString("JoinedStr()")
		}
		return
	}
	d.b.WriteString("JoinedStr(values=[")
	for i, v := range values {
		if i > 0 {
			d.b.WriteString(", ")
		}
		d.adExpr(v, "Load")
	}
	d.b.WriteString("])")
}
