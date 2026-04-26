// Package parser is gopapy's original participle-based Python parser.
// It produces a parse tree (rooted at File) which the ast package
// converts to the public AST shape.
//
// # Deprecation notice (v0.2.0)
//
// New consumers should use [github.com/tamnd/gopapy/v2/parser2].
// parser2 covers the full CPython 3.14 grammar, runs ~83x faster,
// and is the active development line. This package (v1) receives
// security and correctness fixes only; no new features will land
// here.
//
// Migration is a one-line import change:
//
//	- "github.com/tamnd/gopapy/v1/parser"
//	+ "github.com/tamnd/gopapy/v2/parser2"
//
// The AST dump format is identical; there are no field renames or
// API removals.
//
// # API surface (stable, no new additions)
//
//   - ParseFile(filename string, src []byte) (*File, error)
//   - ParseString(filename, src string) (*File, error)
//   - ParseExpression(src string) (*Expression, error)
//   - ParseReader(filename string, src []byte) (*File, error)
//
// These signatures are frozen at v0.1.27 and will not change.
package parser
