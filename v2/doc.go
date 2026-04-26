// Package v2 is the new module home for gopapy's parser2 work.
//
// As of v0.1.28 v2 ships an experimental hand-written expression
// parser under `v2/parser2/`. It produces the same `ast.ExprNode`
// shapes that v1 produces, so downstream code (linter, symbols,
// unparse) does not need to know which parser produced the tree.
// Coverage today is small on purpose; the bulk of expression and
// statement coverage arrives across v0.1.29 and v0.1.30 per
// roadmap v8.
//
// v1 stays the maintenance line and continues to ship from
// `github.com/tamnd/gopapy/v1`. Nothing forces a migration. v2 is
// the path for new development; v1 is the path for stability.
package v2
