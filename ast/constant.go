// Package ast holds Python AST node types generated from CPython's
// Parser/Python.asdl. The generator lives in internal/asdlgen.
//
// Node types and field names are frozen as part of the v0.1.0 contract:
// existing nodes will not be renamed or have fields removed within the
// v1 module path. New optional fields and new node variants may land in
// patch releases as upstream Python grows.
//
// FromFile converts a parser.File into a canonical Module. Dump matches
// CPython's ast.dump output. Unparse renders an AST back into Python
// source.
//
//go:generate go run ../internal/asdlgen/cmd/gen-ast
package ast

// ConstantValue wraps the Python `constant` ASDL builtin. Python's ast module
// stores literal values as untyped Python objects (int, float, complex, str,
// bytes, bool, None, Ellipsis); we tag them by Kind so the dumper can render
// them with the right repr form. The name avoids colliding with the
// generated `Constant` AST node, which carries one of these as its value.
type ConstantValue struct {
	Kind  ConstantValueKind
	Int   string  // decimal/hex/oct/bin literal value, exactly as written
	Float float64 // for ConstantFloat
	Imag  float64 // for ConstantComplex (imaginary part)
	Str   string  // for ConstantStr
	Bytes []byte  // for ConstantBytes
	Bool  bool    // for ConstantBool
}

// ConstantValueKind tags the active field of ConstantValue.
type ConstantValueKind uint8

const (
	ConstantNone ConstantValueKind = iota
	ConstantBool
	ConstantInt
	ConstantFloat
	ConstantComplex
	ConstantStr
	ConstantBytes
	ConstantEllipsis
)
