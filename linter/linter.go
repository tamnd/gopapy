// Package linter is a pyflakes-style static analyzer built on the
// gopapy substrate. Each check lives in its own file (check_f401.go,
// check_f811.go, check_f841.go) and consumes the symbol table plus,
// when needed, the AST itself. No internals of other packages are
// reached into; if a check requires data the public API doesn't
// expose, that's a signal to extend the substrate first.
//
// The two entry points mirror the symbols package: Lint takes a
// parsed module, LintFile takes raw source. Only LintFile honours
// `# noqa` suppression because suppression needs the comment trivia
// the parser drops.
package linter

import (
	"sort"

	"github.com/tamnd/gopapy/v1/ast"
	"github.com/tamnd/gopapy/v1/cst"
	"github.com/tamnd/gopapy/v1/diag"
	"github.com/tamnd/gopapy/v1/symbols"
)

// Stable diagnostic codes. The F prefix matches pyflakes for
// recognisability; codes are zero-padded three digits and do not get
// reused once retired.
const (
	CodeUnusedImport       = "F401" // `import X` whose name is never read
	CodeRedefinitionUnused = "F811" // name rebound without intervening use
	CodeUnusedLocal        = "F841" // local assigned but never read
)

// Lint runs every check on mod and returns diagnostics in stable
// (line, col, code) order. # noqa suppression is not applied — call
// LintFile when you want it.
func Lint(mod *ast.Module) []diag.Diagnostic {
	sm := symbols.Build(mod)
	var out []diag.Diagnostic
	out = append(out, checkF401(sm, mod)...)
	out = append(out, checkF811(sm)...)
	out = append(out, checkF841(sm, mod)...)
	sortDiagnostics(out)
	return out
}

// LintFile parses src, runs every check, applies # noqa suppression
// based on trailing comments, and stamps Filename on each diagnostic.
func LintFile(filename string, src []byte) ([]diag.Diagnostic, error) {
	cf, err := cst.Parse(filename, src)
	if err != nil {
		return nil, err
	}
	diags := Lint(cf.AST)
	noqa := buildNoqaIndex(cf)
	out := diags[:0]
	for _, d := range diags {
		if noqa.suppresses(d.Pos.Lineno, d.Code) {
			continue
		}
		d.Filename = filename
		out = append(out, d)
	}
	return out, nil
}

// sortDiagnostics orders by line, column, then code. Stable so a
// fixed file always produces the same byte sequence.
func sortDiagnostics(ds []diag.Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.Pos.Lineno != b.Pos.Lineno {
			return a.Pos.Lineno < b.Pos.Lineno
		}
		if a.Pos.ColOffset != b.Pos.ColOffset {
			return a.Pos.ColOffset < b.Pos.ColOffset
		}
		return a.Code < b.Code
	})
}

// posBefore is the strict less-than for ast.Pos used by F811's
// "intervening use" check.
func posBefore(a, b ast.Pos) bool {
	if a.Lineno != b.Lineno {
		return a.Lineno < b.Lineno
	}
	return a.ColOffset < b.ColOffset
}
