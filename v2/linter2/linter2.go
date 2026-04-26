// Package linter2 is a pyflakes-style static analyzer built on parser2
// and symbols2. Ten checks ported from v1 linter plus W605 from raw
// source. Entry points: Lint(mod) for AST-only checks, LintFile for
// all checks including the source-aware W605.
package linter2

import (
	"sort"

	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
	"github.com/tamnd/gopapy/v2/symbols2"
)

const (
	CodeUnusedImport               = "F401"
	CodePercentFormatMismatch      = "F501"
	CodeFStringWithoutPlaceholders = "F541"
	CodeIsWithLiteral              = "F632"
	CodeRedefinitionUnused         = "F811"
	CodeUndefinedName              = "F821"
	CodeUnusedLocal                = "F841"
	CodeComparisonToNone           = "E711"
	CodeComparisonToBool           = "E712"
	CodeInvalidEscape              = "W605"
)

// Lint runs every AST-level check on mod. W605 (which needs raw
// source bytes) is not run; call LintFile for full coverage.
func Lint(mod *parser2.Module) []diag.Diagnostic {
	sm := symbols2.Build(mod)
	var out []diag.Diagnostic
	out = append(out, checkF401(sm, mod)...)
	out = append(out, checkF501(mod)...)
	out = append(out, checkF541(mod)...)
	out = append(out, checkF632(mod)...)
	out = append(out, checkF811(sm)...)
	out = append(out, checkF821(sm, mod)...)
	out = append(out, checkF841(sm, mod)...)
	out = append(out, checkE711(mod)...)
	out = append(out, checkE712(mod)...)
	sortDiagnostics(out)
	return out
}

// LintFile parses src, runs all checks including W605, and stamps
// Filename on each diagnostic.
func LintFile(filename string, src []byte) ([]diag.Diagnostic, error) {
	mod, err := parser2.ParseFile(filename, string(src))
	if err != nil {
		return nil, err
	}
	out := Lint(mod)
	out = append(out, checkW605(src)...)
	sortDiagnostics(out)
	for i := range out {
		out[i].Filename = filename
	}
	return out, nil
}

func sortDiagnostics(ds []diag.Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.Pos.Line != b.Pos.Line {
			return a.Pos.Line < b.Pos.Line
		}
		if a.Pos.Col != b.Pos.Col {
			return a.Pos.Col < b.Pos.Col
		}
		return a.Code < b.Code
	})
}
