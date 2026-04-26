// Package parser is gopapy's original participle-based Python parser.
// It produces a parse tree (rooted at File) which the ast package
// converts to the public AST shape.
//
// # v0.1 freeze
//
// As of v0.1.27 this package is feature-frozen. Bug fixes still ship
// from this module path; new constructs and the hand-written
// recursive-descent rewrite (parser2) target the separate
// `github.com/tamnd/gopapy/v2` module path that lands in v0.1.28+.
//
// Downstream consumers staying on `github.com/tamnd/gopapy/v1` keep
// the AST shape they have today. The v2 module ships in parallel
// with v1; nothing forces a migration. v2 is the path for new
// development and the eventual home of formatter and type-checker
// work; v1 is the path for stability.
//
// # API surface
//
// Four exported entry points cover every public consumer:
//
//   - ParseFile(filename string, src []byte) (*File, error)
//   - ParseString(filename, src string) (*File, error)
//   - ParseExpression(src string) (*Expression, error)
//   - ParseReader(filename string, src []byte) (*File, error)
//
// `File` and the rest of the type tree (Statement, Expression, ...)
// form the public AST shape. They are part of the v0.1.0 contract
// and do not change without a v2-style major bump.
package parser
