// Package v2 is the recommended module for new gopapy consumers.
//
// As of v0.2.0 the hand-written parser2 package covers the full
// CPython 3.14 grammar (85/85 grammar fixtures), runs ~83x faster
// than v1 on file parsing, and is the active development line.
//
// Import:
//
//	import "github.com/tamnd/gopapy/v2/parser2"
//
// Entry points:
//
//	parser2.ParseFile(filename, src string) (*Module, error)
//	parser2.ParseExpression(src string) (Expr, error)
//	parser2.DumpModule(*Module) string
//	parser2.Dump(Expr) string
//
// v1 (`github.com/tamnd/gopapy/v1`) receives security and
// correctness fixes only. All new features target v2.
package v2
