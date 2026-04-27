package linter2

import (
	"strings"

	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

func checkW291(src []byte) []diag.Diagnostic {
	var out []diag.Diagnostic
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		// Find how many trailing spaces/tabs the line has.
		trimmed := strings.TrimRight(line, " \t")
		if len(trimmed) == len(line) {
			continue
		}
		lineno := i + 1
		col := len(trimmed)
		out = append(out, diag.Diagnostic{
			Pos:      parser2.Pos{Line: lineno, Col: col},
			End:      parser2.Pos{Line: lineno, Col: len(line)},
			Severity: diag.SeverityWarning,
			Code:     CodeTrailingWhitespace,
			Msg:      "trailing whitespace",
		})
	}
	return out
}
