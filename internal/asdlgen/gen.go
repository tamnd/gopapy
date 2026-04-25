package asdlgen

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
)

// Generate renders the Go AST package source from a parsed Module.
// It emits four files keyed by basename:
//
//	"nodes_gen.go"  - struct per constructor, interface per sum type
//	"visit_gen.go"  - Walk + Inspect helpers
//	"dump_gen.go"   - _fields and _attributes tables for ast.dump parity
//
// The output is gofmt-formatted; an error is returned if formatting fails.
func Generate(m *Module, pkg string) (map[string][]byte, error) {
	idx := buildIndex(m)
	out := map[string][]byte{}
	for name, fn := range map[string]func(*Module, *index, string) string{
		"nodes_gen.go": genNodes,
		"visit_gen.go": genVisit,
		"dump_gen.go":  genDump,
	} {
		src := fn(m, idx, pkg)
		formatted, err := format.Source([]byte(src))
		if err != nil {
			return nil, fmt.Errorf("asdlgen: format %s: %w\n--- raw ---\n%s", name, err, src)
		}
		out[name] = formatted
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Indexing
// ---------------------------------------------------------------------------

type defKind int

const (
	defSum defKind = iota
	defProduct
)

// index lets the generator answer "is this typename a sum or a product?".
type index struct {
	kind map[string]defKind
}

func buildIndex(m *Module) *index {
	idx := &index{kind: map[string]defKind{}}
	for _, d := range m.Defs {
		if d.IsProduct {
			idx.kind[d.Name] = defProduct
		} else {
			idx.kind[d.Name] = defSum
		}
	}
	return idx
}

func (i *index) isSum(t string) bool {
	k, ok := i.kind[t]
	return ok && k == defSum
}

// ---------------------------------------------------------------------------
// Naming
// ---------------------------------------------------------------------------

// goTypeName converts an ASDL type name into a Go exported identifier.
// e.g. expr → Expr, expr_context → ExprContext, type_ignore → TypeIgnore.
func goTypeName(s string) string { return camel(s, true) }

// goFieldName converts an ASDL field name into a Go exported identifier.
// e.g. decorator_list → DecoratorList, is_async → IsAsync.
func goFieldName(s string) string { return camel(s, true) }

func camel(s string, exported bool) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 && !exported {
			parts[i] = strings.ToLower(p)
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

// goFieldType returns the Go type for a field given the ASDL type, optionality,
// and sequence flags.
func goFieldType(f *Field, idx *index) string {
	base := mapBuiltin(f.Type)
	if base == "" {
		// User-defined type. Sum types become interfaces; product types become
		// pointers. Either way the empty value is nil, which is what we want
		// for "optional".
		if idx.isSum(f.Type) {
			base = sumIfaceName(f.Type)
		} else {
			base = "*" + goTypeName(f.Type)
		}
	}
	switch {
	case f.OptSeq:
		// expr?* : slice whose elements can be nil. Only the slice marker shows
		// here; nilness is documented by the field comment.
		return "[]" + base
	case f.Seq:
		return "[]" + base
	case f.Opt:
		// For builtins (string/int) we keep the bare value; absence is signalled
		// out-of-band by an Has* helper or, where convenient, a sentinel "".
		// For user types we already use the nilable form (interface or *T).
		if isBuiltinScalar(f.Type) {
			return base
		}
		return base
	}
	return base
}

func isBuiltinScalar(t string) bool {
	switch t {
	case "identifier", "int", "string", "constant":
		return true
	}
	return false
}

func mapBuiltin(t string) string {
	switch t {
	case "identifier", "string":
		return "string"
	case "int":
		return "int"
	case "constant":
		return "ConstantValue"
	}
	return ""
}

// sumIfaceName returns the Go interface name for a sum-type ASDL name.
// We suffix `Node` so that sum-type interfaces can never collide with
// constructor struct names (e.g. expr→ExprNode vs Expr stmt constructor;
// mod→ModNode vs Mod operator constructor; type_ignore→TypeIgnoreNode vs
// TypeIgnore constructor).
func sumIfaceName(asdlName string) string { return goTypeName(asdlName) + "Node" }

// ---------------------------------------------------------------------------
// File: nodes_gen.go
// ---------------------------------------------------------------------------

func genNodes(m *Module, idx *index, pkg string) string {
	var b strings.Builder
	writeHeader(&b, pkg)
	b.WriteString(`
// Pos carries source positions copied directly from the lexer. CPython uses
// 1-indexed lineno and 0-indexed UTF-8 byte col_offset; we match that
// exactly so ast.dump output is byte-equal.
type Pos struct {
	Lineno       int
	ColOffset    int
	EndLineno    int
	EndColOffset int
}

// pos returns p (used by generated GetPos methods).
func (p Pos) pos() Pos { return p }
`)

	defs := append([]*Def(nil), m.Defs...)
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })

	for _, d := range defs {
		if d.IsProduct {
			emitProduct(&b, d, idx)
		} else {
			emitSum(&b, d, idx)
		}
	}
	return b.String()
}

func writeHeader(b *strings.Builder, pkg string) {
	fmt.Fprintf(b, `// Code generated by internal/asdlgen. DO NOT EDIT.
// Source: internal/asdlgen/Python.asdl

package %s

`, pkg)
}

// emitSum writes:
//   - one interface for the sum type (marker method tagSum_<name>)
//   - one struct per constructor implementing the marker
//   - if the sum has attributes, an embedded Pos and a GetPos accessor
func emitSum(b *strings.Builder, d *Def, idx *index) {
	iface := sumIfaceName(d.Name)
	marker := "is" + iface
	hasAttrs := len(d.Attributes) > 0
	fmt.Fprintf(b, "\n// %s is the ASDL sum type `%s` (one of %d constructors).\n",
		iface, d.Name, len(d.Constructors))
	fmt.Fprintf(b, "type %s interface {\n\t%s()\n", iface, marker)
	if hasAttrs {
		b.WriteString("\tGetPos() Pos\n")
	}
	b.WriteString("}\n")

	for _, c := range d.Constructors {
		emitConstructor(b, iface, marker, c, d.Attributes, idx)
	}
}

// emitConstructor writes one struct + its marker method (and GetPos if attrs).
func emitConstructor(b *strings.Builder, iface, marker string, c *Constructor, attrs []*Field, idx *index) {
	name := goTypeName(c.Name)
	fmt.Fprintf(b, "\n// %s is the `%s` constructor of %s.\n", name, c.Name, iface)
	fmt.Fprintf(b, "type %s struct {\n", name)
	if len(attrs) > 0 {
		b.WriteString("\tPos\n")
	}
	for _, f := range c.Fields {
		fmt.Fprintf(b, "\t%s %s // %s\n", goFieldName(f.Name), goFieldType(f, idx), f.String())
	}
	b.WriteString("}\n")
	fmt.Fprintf(b, "func (*%s) %s() {}\n", name, marker)
	if len(attrs) > 0 {
		fmt.Fprintf(b, "func (n *%s) GetPos() Pos { return n.Pos }\n", name)
	}
}

// emitProduct writes a single struct for an ASDL product type. Product types
// can carry attributes too (e.g. arg, keyword, alias all have positions).
func emitProduct(b *strings.Builder, d *Def, idx *index) {
	name := goTypeName(d.Name)
	fmt.Fprintf(b, "\n// %s is the ASDL product type `%s`.\n", name, d.Name)
	fmt.Fprintf(b, "type %s struct {\n", name)
	if len(d.Attributes) > 0 {
		b.WriteString("\tPos\n")
	}
	for _, f := range d.Fields {
		fmt.Fprintf(b, "\t%s %s // %s\n", goFieldName(f.Name), goFieldType(f, idx), f.String())
	}
	b.WriteString("}\n")
	if len(d.Attributes) > 0 {
		fmt.Fprintf(b, "func (n *%s) GetPos() Pos { return n.Pos }\n", name)
	}
}

// ---------------------------------------------------------------------------
// File: visit_gen.go
// ---------------------------------------------------------------------------

// genVisit emits a Walk(any, func(any) bool) that recurses into every AST node
// field by type assertion. The visitor returns false to stop descent into a
// node, mirroring go/ast.Walk semantics.
func genVisit(m *Module, idx *index, pkg string) string {
	var b strings.Builder
	writeHeader(&b, pkg)
	b.WriteString(`
// Node is the union of every AST node type produced by the generator.
// Concrete sum-type interfaces (Stmt, Expr, ...) all satisfy Node.
type Node interface{}

// Walk traverses an AST in depth-first order. For each non-nil node, it calls
// fn(n); if fn returns false, the children of n are skipped.
func Walk(n Node, fn func(Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	walkChildren(n, fn)
}
`)
	b.WriteString("\nfunc walkChildren(n Node, fn func(Node) bool) {\n")
	b.WriteString("\tswitch n := n.(type) {\n")

	defs := append([]*Def(nil), m.Defs...)
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })

	for _, d := range defs {
		if d.IsProduct {
			emitVisitProduct(&b, d, idx)
		} else {
			for _, c := range d.Constructors {
				emitVisitConstructor(&b, c, idx)
			}
		}
	}
	b.WriteString("\t}\n}\n")
	return b.String()
}

func emitVisitProduct(b *strings.Builder, d *Def, idx *index) {
	emitVisitFields(b, goTypeName(d.Name), d.Fields, idx)
}

func emitVisitConstructor(b *strings.Builder, c *Constructor, idx *index) {
	emitVisitFields(b, goTypeName(c.Name), c.Fields, idx)
}

func emitVisitFields(b *strings.Builder, name string, fields []*Field, idx *index) {
	if !hasNodeChild(fields, idx) {
		// No node-typed fields: emit nothing — the type-switch case can be
		// omitted entirely, falling through to the default no-op.
		return
	}
	fmt.Fprintf(b, "\tcase *%s:\n", name)
	for _, f := range fields {
		if isBuiltinScalar(f.Type) {
			continue
		}
		field := "n." + goFieldName(f.Name)
		switch {
		case f.Seq, f.OptSeq:
			fmt.Fprintf(b, "\t\tfor _, c := range %s { Walk(c, fn) }\n", field)
		default:
			fmt.Fprintf(b, "\t\tWalk(%s, fn)\n", field)
		}
	}
}

func hasNodeChild(fields []*Field, idx *index) bool {
	_ = idx
	for _, f := range fields {
		if !isBuiltinScalar(f.Type) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// File: dump_gen.go
// ---------------------------------------------------------------------------

// genDump emits a metadata table that the dump and json passes use to render
// nodes the way ast.dump does: by walking _fields in declared order.
func genDump(m *Module, idx *index, pkg string) string {
	_ = idx
	var b strings.Builder
	writeHeader(&b, pkg)
	b.WriteString(`
// FieldInfo describes one field on an AST node, in the order CPython's
// _fields tuple lists them. Kind tells the renderer how to walk the value:
// scalar fields print their Go value; node fields recurse; seq fields render
// as Python lists.
type FieldInfo struct {
	Name   string // ASDL/Python field name (e.g. "decorator_list")
	GoName string // Go field name           (e.g. "DecoratorList")
	Kind   FieldKind
}

type FieldKind uint8

const (
	FieldScalar    FieldKind = iota // string, int, Constant
	FieldNode                       // single AST node (interface or *Struct)
	FieldSeq                        // []Node
	FieldOptSeq                     // []Node where elements may be nil (Dict.keys)
)

// NodeInfo holds the dump metadata for one constructor or product type.
type NodeInfo struct {
	PyName     string      // e.g. "BinOp"
	Fields     []FieldInfo // _fields tuple order
	Attributes []FieldInfo // _attributes tuple, if any
}

// nodeInfoTable is keyed by Go type name (without the package prefix).
var nodeInfoTable = map[string]*NodeInfo{
`)

	defs := append([]*Def(nil), m.Defs...)
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })

	for _, d := range defs {
		if d.IsProduct {
			emitDumpEntry(&b, goTypeName(d.Name), d.Name, d.Fields, d.Attributes)
		} else {
			for _, c := range d.Constructors {
				emitDumpEntry(&b, goTypeName(c.Name), c.Name, c.Fields, d.Attributes)
			}
		}
	}
	b.WriteString("}\n")
	b.WriteString(`
// LookupNodeInfo returns the dump metadata for a Go type name (e.g. "BinOp").
func LookupNodeInfo(goName string) *NodeInfo { return nodeInfoTable[goName] }
`)
	return b.String()
}

func emitDumpEntry(b *strings.Builder, goName, pyName string, fields, attrs []*Field) {
	fmt.Fprintf(b, "\t%q: {\n\t\tPyName: %q,\n", goName, pyName)
	if len(fields) > 0 {
		b.WriteString("\t\tFields: []FieldInfo{\n")
		for _, f := range fields {
			fmt.Fprintf(b, "\t\t\t{Name: %q, GoName: %q, Kind: %s},\n",
				f.Name, goFieldName(f.Name), kindFor(f))
		}
		b.WriteString("\t\t},\n")
	}
	if len(attrs) > 0 {
		b.WriteString("\t\tAttributes: []FieldInfo{\n")
		for _, f := range attrs {
			fmt.Fprintf(b, "\t\t\t{Name: %q, GoName: %q, Kind: FieldScalar},\n",
				f.Name, goFieldName(f.Name))
		}
		b.WriteString("\t\t},\n")
	}
	b.WriteString("\t},\n")
}

func kindFor(f *Field) string {
	switch {
	case f.OptSeq:
		return "FieldOptSeq"
	case f.Seq:
		return "FieldSeq"
	case isBuiltinScalar(f.Type):
		return "FieldScalar"
	default:
		return "FieldNode"
	}
}

// EnsureGofmt is exposed for the cmd/asdlgen tool to format external files.
func EnsureGofmt(src []byte) ([]byte, error) {
	return format.Source(src)
}

// stub to keep bytes import alive when the generator grows.
var _ = bytes.MinRead
