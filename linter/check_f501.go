package linter

import (
	"strconv"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/diag"
)

// checkF501 fires on `"<fmt>" % args` where the conversion-count in
// the format string disagrees with the right-hand side. Only handles
// the two common shapes:
//
//   - Right is a Tuple: len(elts) must equal the conversion count.
//   - Right is anything else (a single value): conversion count must
//     be exactly 1.
//
// Right is a Dict: skipped entirely. `%(name)s` formatting uses keyed
// conversions; the substring count doesn't apply and pyflakes covers
// it under a separate code (F502) we haven't shipped yet.
//
// Conversion counting is intentionally simple: any `%X` where X is in
// the conversion alphabet counts as one argument, and `%%` is treated
// as a literal escape. Format flags / width / precision are not
// parsed because pyflakes itself doesn't parse them either — the rule
// is "count %-codes that need an argument".
func checkF501(mod *ast.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	ast.WalkPreorder(mod, func(n ast.Node) {
		bo, ok := n.(*ast.BinOp)
		if !ok {
			return
		}
		if _, ok := bo.Op.(*ast.Mod); !ok {
			return
		}
		lit, ok := bo.Left.(*ast.Constant)
		if !ok || lit.Value.Kind != ast.ConstantStr {
			return
		}
		want, okCount := countPercentConversions(lit.Value.Str)
		if !okCount {
			return
		}
		var got int
		switch r := bo.Right.(type) {
		case *ast.Dict:
			return
		case *ast.Tuple:
			got = len(r.Elts)
		default:
			got = 1
		}
		if got == want {
			return
		}
		out = append(out, diag.Diagnostic{
			Pos:      bo.Pos,
			End:      bo.Pos,
			Severity: diag.SeverityWarning,
			Code:     CodePercentFormatMismatch,
			Msg:      percentMismatchMsg(want, got),
		})
	})
	return out
}

// countPercentConversions returns the number of argument-consuming
// `%X` codes in s, treating `%%` as a literal. The second return is
// false when the string contains a `%(name)X` keyed conversion — those
// are skipped at the call site since they need dict-key matching, not
// positional counting.
func countPercentConversions(s string) (int, bool) {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		i++
		if i >= len(s) {
			return 0, false
		}
		if s[i] == '%' {
			continue
		}
		if s[i] == '(' {
			return 0, false
		}
		// Skip flags / width / precision until we hit a conversion char.
		for i < len(s) && !isPercentConversionChar(s[i]) {
			i++
		}
		if i >= len(s) {
			return 0, false
		}
		n++
	}
	return n, true
}

// isPercentConversionChar reports whether c is a Python printf-style
// conversion type. Pyflakes counts each one of these (other than the
// `%%` escape handled separately) as needing one argument.
func isPercentConversionChar(c byte) bool {
	switch c {
	case 'd', 'i', 'o', 'u', 'x', 'X',
		'e', 'E', 'f', 'F', 'g', 'G',
		'c', 'r', 's', 'a':
		return true
	}
	return false
}

func percentMismatchMsg(want, got int) string {
	verb := "arguments"
	if want == 1 {
		verb = "argument"
	}
	return "%-format string expects " + strconv.Itoa(want) + " " + verb +
		", got " + strconv.Itoa(got)
}
