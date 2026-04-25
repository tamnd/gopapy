package ast

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

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
		b.WriteString(strconv.Quote(s))
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
		b.WriteString(strconv.FormatFloat(c.Float, 'g', -1, 64))
	case ConstantComplex:
		b.WriteString(strconv.FormatFloat(c.Imag, 'g', -1, 64))
		b.WriteByte('j')
	case ConstantStr:
		b.WriteString(strconv.Quote(c.Str))
	case ConstantBytes:
		b.WriteByte('b')
		b.WriteString(strconv.Quote(string(c.Bytes)))
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
