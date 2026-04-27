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

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/cst"
	"github.com/tamnd/gopapy/legacy/diag"
	"github.com/tamnd/gopapy/legacy/symbols"
)

// Stable diagnostic codes. The F prefix matches pyflakes and the
// E prefix matches pycodestyle for recognisability; codes are
// zero-padded three digits and do not get reused once retired.
const (
	CodeUnusedImport               = "F401" // `import X` whose name is never read
	CodePercentFormatMismatch      = "F501" // `%`-format with mismatched argument count
	CodeFStringWithoutPlaceholders = "F541" // f-string with no `{...}` interpolation
	CodeIsWithLiteral              = "F632" // `is` / `is not` against a literal value
	CodeRedefinitionUnused         = "F811" // name rebound without intervening use
	CodeUndefinedName              = "F821" // reference to a name no scope binds
	CodeUnusedLocal                = "F841" // local assigned but never read
	CodeComparisonToNone           = "E711" // `== None` / `!= None` instead of `is`
	CodeComparisonToBool           = "E712" // `== True` / `== False` instead of `is`
	CodeInvalidEscape              = "W605" // `\p` and friends in non-raw string literals
)

// Lint runs every check on mod and returns diagnostics in stable
// (line, col, code) order. # noqa suppression is not applied — call
// LintFile when you want it. Equivalent to LintWithConfig(mod, Config{}).
func Lint(mod *ast.Module) []diag.Diagnostic {
	return LintWithConfig(mod, Config{})
}

// LintWithConfig runs every check enabled by cfg on mod. Per-file
// ignores in cfg are not applied here because Lint operates on a bare
// module without a filename; LintFileWithConfig is the path that
// honours them.
func LintWithConfig(mod *ast.Module, cfg Config) []diag.Diagnostic {
	sm := symbols.Build(mod)
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
	return filterEnabled(out, cfg, "")
}

// LintFile parses src, runs every check, applies # noqa suppression
// based on trailing comments, and stamps Filename on each diagnostic.
// Equivalent to LintFileWithConfig(filename, src, Config{}).
func LintFile(filename string, src []byte) ([]diag.Diagnostic, error) {
	return LintFileWithConfig(filename, src, Config{})
}

// LintFileWithConfig parses src, runs every check enabled by cfg
// (including per-file ignores keyed off filename), applies # noqa
// suppression, and stamps Filename on each surviving diagnostic.
//
// Source-aware checks (those that need raw bytes — W605 and any
// future siblings) run here too, against the cst.File. They never
// run via Lint(mod) because there's no source to give them; that's
// a documented limitation, callers wanting full coverage use
// LintFile.
func LintFileWithConfig(filename string, src []byte, cfg Config) ([]diag.Diagnostic, error) {
	cf, err := cst.Parse(filename, src)
	if err != nil {
		return nil, err
	}
	diags := LintWithConfig(cf.AST, cfg)
	diags = append(diags, runSourceChecks(cf, cfg)...)
	sortDiagnostics(diags)
	noqa := buildNoqaIndex(cf)
	out := diags[:0]
	for _, d := range diags {
		if noqa.suppresses(d.Pos.Lineno, d.Code) {
			continue
		}
		if !cfg.EnabledFor(filename, d.Code) {
			continue
		}
		d.Filename = filename
		out = append(out, d)
	}
	return out, nil
}

// runSourceChecks runs every check that needs raw source bytes
// (cst.File.Tokens / cst.File.Source). Each check is gated on its
// own code being enabled under cfg; per-file ignores apply later in
// the LintFileWithConfig filtering pass.
func runSourceChecks(cf *cst.File, cfg Config) []diag.Diagnostic {
	var out []diag.Diagnostic
	if cfg.Enabled(CodeInvalidEscape) {
		out = append(out, checkW605(cf)...)
	}
	return out
}

// filterEnabled drops diagnostics whose code is not enabled under
// cfg. When filename is non-empty, per-file ignores are also applied;
// pass "" to skip the per-file pass (Lint without a filename).
func filterEnabled(diags []diag.Diagnostic, cfg Config, filename string) []diag.Diagnostic {
	out := diags[:0]
	for _, d := range diags {
		if filename == "" {
			if !cfg.Enabled(d.Code) {
				continue
			}
		} else if !cfg.EnabledFor(filename, d.Code) {
			continue
		}
		out = append(out, d)
	}
	return out
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
