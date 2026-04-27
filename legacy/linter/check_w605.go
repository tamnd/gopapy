package linter

import (
	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/cst"
	"github.com/tamnd/gopapy/legacy/diag"
	"github.com/tamnd/gopapy/lex"
)

// checkW605 fires on `\X` in a string literal where X is not a
// recognized escape character. Pure source-byte check: the AST hides
// the spelling (`"\p"` and `"\\p"` fold to the same Constant), so
// the check has to read the raw token value.
//
// The substrate gives us this for free via cst.File.Tokens(): each
// STRING token's Value is the lexeme exactly as it appeared in source,
// including prefix letters (`r`, `b`, `f`, etc.) and quote chars.
//
// Skipped:
//   - raw strings (`r"\p"`, `R"\p"`) — backslashes are literal in raw
//     strings, so the escape is by definition fine.
//   - characters inside `{...}` placeholders of f/t-strings — those
//     are expressions/format specs, not string literals.
//
// The diagnostic Pos is the start of the string token, not the offset
// of the bad escape. pycodestyle does the same; column-level
// positioning inside multi-line strings adds complexity for marginal
// payoff.
func checkW605(cf *cst.File) []diag.Diagnostic {
	if cf == nil {
		return nil
	}
	var out []diag.Diagnostic
	for _, tok := range cf.Tokens() {
		if tok.Kind != lex.STRING {
			continue
		}
		body, isRaw, isBytes, isFOrT, ok := splitStringLiteral(tok.Value)
		if !ok || isRaw {
			continue
		}
		if !scanForBadEscape(body, isBytes, isFOrT) {
			continue
		}
		out = append(out, diag.Diagnostic{
			Pos:      ast.Pos{Lineno: tok.Pos.Line, ColOffset: tok.Pos.Col},
			End:      ast.Pos{Lineno: tok.End.Line, ColOffset: tok.End.Col},
			Severity: diag.SeverityWarning,
			Code:     CodeInvalidEscape,
			Msg:      "invalid escape sequence",
		})
	}
	return out
}

// splitStringLiteral parses prefix + quote + body + quote out of the
// raw lexeme. Returns the body (between the quotes), prefix flags,
// and ok=false if the input doesn't look like a string literal at all
// (which shouldn't happen for a STRING token but defensiveness is
// cheap here).
func splitStringLiteral(s string) (body string, isRaw, isBytes, isFOrT, ok bool) {
	i := 0
	for i < len(s) {
		c := s[i]
		switch c {
		case 'r', 'R':
			isRaw = true
		case 'b', 'B':
			isBytes = true
		case 'f', 'F', 't', 'T':
			isFOrT = true
		case 'u', 'U':
			// `u`-prefix is a no-op in Py3; just consume.
		case '"', '\'':
			// Quote chars start the body.
			goto Quote
		default:
			return "", false, false, false, false
		}
		i++
	}
	return "", false, false, false, false
Quote:
	if i >= len(s) {
		return "", false, false, false, false
	}
	q := s[i]
	// Triple-quoted form: """...""" or '''...'''.
	if i+2 < len(s) && s[i+1] == q && s[i+2] == q {
		if len(s) < i+6 {
			return "", false, false, false, false
		}
		return s[i+3 : len(s)-3], isRaw, isBytes, isFOrT, true
	}
	if len(s) < i+2 {
		return "", false, false, false, false
	}
	return s[i+1 : len(s)-1], isRaw, isBytes, isFOrT, true
}

// scanForBadEscape returns true if body contains at least one
// invalid `\X` escape outside any `{...}` placeholder. For non-f/t
// strings, depth always stays zero. The recognized-escape set
// matches CPython 3.14: standard short escapes, octal, hex, plus
// the str-only `\N`, `\u`, `\U` family (which are errors in bytes
// literals).
func scanForBadEscape(body string, isBytes, isFOrT bool) bool {
	depth := 0
	for i := 0; i < len(body); i++ {
		c := body[i]
		if isFOrT {
			switch c {
			case '{':
				if i+1 < len(body) && body[i+1] == '{' {
					i++
					continue
				}
				depth++
				continue
			case '}':
				if depth == 0 {
					if i+1 < len(body) && body[i+1] == '}' {
						i++
					}
					continue
				}
				depth--
				continue
			}
			if depth > 0 {
				continue
			}
		}
		if c != '\\' {
			continue
		}
		if i+1 >= len(body) {
			// Trailing lone backslash: not our concern (CPython
			// raises SyntaxError on this in non-raw strings, so
			// we'd never see one in valid source).
			return false
		}
		next := body[i+1]
		if isRecognizedEscape(next, isBytes) {
			i += escapeAdvance(body, i, isBytes)
			continue
		}
		return true
	}
	return false
}

// isRecognizedEscape reports whether `\X` is a valid escape. The
// bool flag selects the bytes-vs-str rule set: `\N`, `\u`, `\U` are
// str-only (error in bytes literals).
func isRecognizedEscape(c byte, isBytes bool) bool {
	switch c {
	case '\\', '\'', '"', 'a', 'b', 'f', 'n', 'r', 't', 'v', '0', '1', '2', '3', '4', '5', '6', '7', 'x', '\n', '\r':
		return true
	case 'N', 'u', 'U':
		return !isBytes
	}
	return false
}

// escapeAdvance returns how many extra bytes to skip after the `\`
// at position i so the outer scan doesn't re-examine the consumed
// escape. Returns 0 when we want the standard `i++ then i++ in loop`
// (skip just `\` and X). Returns more when the escape consumes
// trailing chars (e.g. `\xHH` consumes two more hex digits).
//
// The loop's own `i++` advances past the `\`; this return advances
// past the X plus any trailing digits.
func escapeAdvance(body string, i int, isBytes bool) int {
	c := body[i+1]
	switch c {
	case 'x':
		// `\xHH` — two hex digits. Consume them so a sequence like
		// `\x41x` doesn't trip a false `\x` re-read.
		return 1 + minHexDigits(body, i+2, 2)
	case '0', '1', '2', '3', '4', '5', '6', '7':
		// Octal — consume up to two more octal digits.
		return 1 + minOctalDigits(body, i+2, 2)
	case 'u':
		if !isBytes {
			return 1 + minHexDigits(body, i+2, 4)
		}
	case 'U':
		if !isBytes {
			return 1 + minHexDigits(body, i+2, 8)
		}
	case 'N':
		if !isBytes {
			// `\N{NAME}` — skip past closing brace if present.
			if i+2 < len(body) && body[i+2] == '{' {
				j := i + 3
				for j < len(body) && body[j] != '}' {
					j++
				}
				if j < len(body) {
					return j - i // points at `}`; outer loop's i++ moves past it.
				}
			}
		}
	}
	return 1
}

func minHexDigits(s string, start, max int) int {
	n := 0
	for n < max && start+n < len(s) {
		c := s[start+n]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			n++
			continue
		}
		break
	}
	return n
}

func minOctalDigits(s string, start, max int) int {
	n := 0
	for n < max && start+n < len(s) {
		c := s[start+n]
		if c >= '0' && c <= '7' {
			n++
			continue
		}
		break
	}
	return n
}
