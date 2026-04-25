package ast

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// pyFloatRepr matches CPython's float repr: fixed notation for values in
// [1e-4, 1e16), scientific outside, and an explicit ".0" when the fixed
// form has no fractional part. NaN/Inf follow Python's "nan"/"inf" form.
func pyFloatRepr(f float64) string {
	if math.IsNaN(f) {
		return "nan"
	}
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	abs := math.Abs(f)
	if abs < 1e-4 || abs >= 1e16 {
		return strconv.FormatFloat(f, 'e', -1, 64)
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// pyComplexRepr matches CPython's repr for the imaginary part of a complex
// constant: integral values show as `1j`, not `1.0j`. Otherwise it follows
// the float repr rules.
func pyComplexRepr(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		abs := math.Abs(f)
		if abs == 0 || (abs >= 1e-4 && abs < 1e16) {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	}
	return pyFloatRepr(f)
}

// Dump renders a node in the same shape CPython's ast.dump(tree) produces.
// The default form is the compact one — matching ast.dump(tree).
//
//	ast.dump(ast.parse("1+2"))
//	# => "Module(body=[Expr(value=BinOp(left=Constant(value=1), op=Add(), right=Constant(value=2)))], type_ignores=[])"
//
// Field order follows the generated nodeInfoTable, which mirrors CPython's
// _fields tuple per ASDL definition. Optional fields with zero values are
// omitted in the same situations as CPython's default rendering.
func Dump(n Node) string {
	var b strings.Builder
	dumpNode(&b, reflect.ValueOf(n))
	return b.String()
}

func dumpNode(b *strings.Builder, v reflect.Value) {
	if !v.IsValid() {
		b.WriteString("None")
		return
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			b.WriteString("None")
			return
		}
		v = v.Elem()
	}
	t := v.Type()
	if t.Kind() != reflect.Struct {
		dumpScalar(b, v)
		return
	}
	if t == reflect.TypeOf(ConstantValue{}) {
		dumpConstant(b, v.Interface().(ConstantValue))
		return
	}

	info := LookupNodeInfo(t.Name())
	if info == nil {
		// Unknown product type (no nodeInfoTable entry). Fall back to
		// printing all exported fields by Go name. This keeps debug
		// output useful even before the generator picks up new types.
		dumpUnknown(b, v)
		return
	}

	pyName := info.PyName
	// PyName for product types is lowercase (e.g. "alias"); for sum
	// constructors it stays as-is. CPython dumps both with their proper
	// constructor name, which equals the Go type name.
	if isSumConstructor(t.Name()) {
		pyName = t.Name()
	}
	b.WriteString(pyName)
	b.WriteByte('(')
	first := true
	for _, f := range info.Fields {
		fv := v.FieldByName(f.GoName)
		if !fv.IsValid() {
			continue
		}
		if f.Optional && isEmptyField(fv, f.Kind) {
			continue
		}
		if !first {
			b.WriteString(", ")
		}
		first = false
		b.WriteString(f.Name)
		b.WriteByte('=')
		switch f.Kind {
		case FieldSeq, FieldOptSeq:
			dumpList(b, fv)
		case FieldScalar:
			dumpScalar(b, fv)
		default:
			dumpNode(b, fv)
		}
	}
	b.WriteByte(')')
}

func dumpList(b *strings.Builder, v reflect.Value) {
	b.WriteByte('[')
	n := v.Len()
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		dumpNode(b, v.Index(i))
	}
	b.WriteByte(']')
}

func dumpScalar(b *strings.Builder, v reflect.Value) {
	if !v.IsValid() {
		b.WriteString("None")
		return
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			b.WriteString("None")
			return
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct && v.Type() == reflect.TypeOf(ConstantValue{}) {
		dumpConstant(b, v.Interface().(ConstantValue))
		return
	}
	switch v.Kind() {
	case reflect.String:
		s := v.String()
		if s == "" {
			b.WriteString("None")
			return
		}
		b.WriteString(pyRepr(s))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Float32, reflect.Float64:
		b.WriteString(strconv.FormatFloat(v.Float(), 'g', -1, 64))
	case reflect.Bool:
		if v.Bool() {
			b.WriteString("True")
		} else {
			b.WriteString("False")
		}
	default:
		fmt.Fprintf(b, "%v", v.Interface())
	}
}

// dumpConstant renders a Python constant the way ast.dump prints it: bare
// ints, strs in quotes, None/True/False/Ellipsis as keywords, complex with
// the trailing j.
func dumpConstant(b *strings.Builder, c ConstantValue) {
	switch c.Kind {
	case ConstantNone:
		b.WriteString("None")
	case ConstantBool:
		if c.Bool {
			b.WriteString("True")
		} else {
			b.WriteString("False")
		}
	case ConstantInt:
		b.WriteString(c.Int)
	case ConstantFloat:
		b.WriteString(pyFloatRepr(c.Float))
	case ConstantComplex:
		b.WriteString(pyComplexRepr(c.Imag))
		b.WriteByte('j')
	case ConstantStr:
		b.WriteString(pyRepr(c.Str))
	case ConstantBytes:
		b.WriteByte('b')
		b.WriteString(pyRepr(string(c.Bytes)))
	case ConstantEllipsis:
		b.WriteString("Ellipsis")
	default:
		b.WriteString("None")
	}
}

func dumpUnknown(b *strings.Builder, v reflect.Value) {
	t := v.Type()
	b.WriteString(t.Name())
	b.WriteByte('(')
	first := true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Anonymous {
			continue
		}
		if !first {
			b.WriteString(", ")
		}
		first = false
		b.WriteString(strings.ToLower(f.Name))
		b.WriteByte('=')
		dumpNode(b, v.Field(i))
	}
	b.WriteByte(')')
}

// pyRepr renders a Go string the way Python's repr() does: single quotes
// when possible, double quotes when the string contains a single quote but
// no double quote, with the usual escapes inside.
func pyRepr(s string) string {
	hasSingle := strings.ContainsRune(s, '\'')
	hasDouble := strings.ContainsRune(s, '"')
	quote := byte('\'')
	if hasSingle && !hasDouble {
		quote = '"'
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
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

// isEmptyField reports whether an optional field's value is the default
// CPython's ast.dump skips: [] for sequences, None for everything else.
func isEmptyField(v reflect.Value, kind FieldKind) bool {
	switch kind {
	case FieldSeq, FieldOptSeq:
		return v.Kind() == reflect.Slice && v.Len() == 0
	case FieldScalar:
		// Optional scalars only appear as plain Go strings in the
		// generated nodes (type_comment, kind, asname, ...). An empty
		// string is how the emitter represents "absent".
		if v.Kind() == reflect.String {
			return v.Len() == 0
		}
		return false
	default:
		// Node fields: nil interface or nil pointer means absent.
		switch v.Kind() {
		case reflect.Interface, reflect.Ptr:
			return v.IsNil()
		}
		return false
	}
}

// isSumConstructor reports whether the given Go type name is a sum-type
// constructor (its info entry has no fields and no attributes — it's just
// a marker like `Add`, `Or`, `Load`). For these CPython prints the bare
// constructor name with empty parens.
func isSumConstructor(goName string) bool {
	info := nodeInfoTable[goName]
	if info == nil {
		return false
	}
	return len(info.Fields) == 0 && len(info.Attributes) == 0
}
