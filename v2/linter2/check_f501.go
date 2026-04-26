package linter2

import (
	"strconv"

	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

func checkF501(mod *parser2.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkModule(mod, func(e parser2.Expr) {
		bo, ok := e.(*parser2.BinOp)
		if !ok || bo.Op != "Mod" {
			return
		}
		lit, ok := bo.Left.(*parser2.Constant)
		if !ok || lit.Kind != "str" {
			return
		}
		s, ok := lit.Value.(string)
		if !ok {
			return
		}
		want, okCount := countPercentConversions(s)
		if !okCount {
			return
		}
		var got int
		switch r := bo.Right.(type) {
		case *parser2.Dict:
			return
		case *parser2.Tuple:
			got = len(r.Elts)
		default:
			got = 1
		}
		if got == want {
			return
		}
		out = append(out, diag.Diagnostic{
			Pos:      bo.P,
			End:      bo.P,
			Severity: diag.SeverityWarning,
			Code:     CodePercentFormatMismatch,
			Msg:      percentMismatchMsg(want, got),
		})
	})
	return out
}

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
