package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
)

// checkW605 scans raw source bytes for invalid escape sequences in
// non-raw string literals. The scan is a lightweight tokenizer that
// finds string literal boundaries and checks each `\X` inside.
func checkW605(src []byte) []diag.Diagnostic {
	var out []diag.Diagnostic
	s := string(src)
	i := 0
	line := 1
	col := 0
	for i < len(s) {
		c := s[i]
		if c == '\n' {
			line++
			col = 0
			i++
			continue
		}
		if c == '#' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		// Detect string prefix + opening quote.
		prefix, quoteStart := scanStringPrefix(s, i)
		if quoteStart < 0 {
			col++
			i++
			continue
		}
		tokLine, tokCol := line, col
		isRaw := false
		isBytes := false
		isFOrT := false
		for _, ch := range prefix {
			switch ch {
			case 'r', 'R':
				isRaw = true
			case 'b', 'B':
				isBytes = true
			case 'f', 'F', 't', 'T':
				isFOrT = true
			}
		}
		// Advance past prefix.
		i += len(prefix)
		col += len(prefix)

		q := s[i]
		triple := i+2 < len(s) && s[i+1] == q && s[i+2] == q
		var body string
		var consumed int
		if triple {
			body, consumed = scanTripleQuoted(s, i+3, q)
			i += 3 + consumed
		} else {
			body, consumed = scanSingleQuoted(s, i+1, q)
			i += 1 + consumed
		}
		col += consumed + 3
		if isRaw {
			// Update line/col for what we consumed.
			for _, ch := range prefix + string(q) + body {
				if ch == '\n' {
					line++
					col = 0
				} else {
					col++
				}
			}
			continue
		}
		if scanForBadEscape(body, isBytes, isFOrT) {
			out = append(out, diag.Diagnostic{
				Pos:      parser.Pos{Line: tokLine, Col: tokCol},
				End:      parser.Pos{Line: tokLine, Col: tokCol},
				Severity: diag.SeverityWarning,
				Code:     CodeInvalidEscape,
				Msg:      "invalid escape sequence",
			})
		}
		// Update line/col counters.
		for _, ch := range body {
			if ch == '\n' {
				line++
				col = 0
			} else {
				col++
			}
		}
	}
	return out
}

// scanStringPrefix returns the prefix letters (r, b, f, t, u) at s[i:]
// and the index of the opening quote, or -1 if s[i] doesn't start a string.
func scanStringPrefix(s string, i int) (string, int) {
	j := i
	for j < len(s) {
		c := s[j]
		switch c {
		case 'r', 'R', 'b', 'B', 'f', 'F', 't', 'T', 'u', 'U':
			j++
			continue
		case '"', '\'':
			if j == i {
				// No prefix, plain quote.
				return "", 0
			}
			return s[i:j], 0
		default:
			return "", -1
		}
	}
	return "", -1
}

// scanSingleQuoted consumes s starting after the opening quote until
// the matching closing quote. Returns the body (excluding quotes) and
// bytes consumed (including the closing quote).
func scanSingleQuoted(s string, start int, q byte) (string, int) {
	i := start
	for i < len(s) {
		c := s[i]
		if c == '\\' {
			i += 2
			continue
		}
		if c == q || c == '\n' {
			return s[start:i], i - start + 1
		}
		i++
	}
	return s[start:], len(s) - start
}

// scanTripleQuoted consumes s starting after the opening triple-quote.
func scanTripleQuoted(s string, start int, q byte) (string, int) {
	end := string([]byte{q, q, q})
	i := start
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		if i+2 < len(s) && s[i] == q && s[i+1] == q && s[i+2] == q {
			return s[start:i], i - start + 3
		}
		i++
	}
	_ = end
	return s[start:], len(s) - start
}

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

func isRecognizedEscape(c byte, isBytes bool) bool {
	switch c {
	case '\\', '\'', '"', 'a', 'b', 'f', 'n', 'r', 't', 'v',
		'0', '1', '2', '3', '4', '5', '6', '7', 'x', '\n', '\r':
		return true
	case 'N', 'u', 'U':
		return !isBytes
	}
	return false
}

func escapeAdvance(body string, i int, isBytes bool) int {
	c := body[i+1]
	switch c {
	case 'x':
		return 1 + minHexDigits(body, i+2, 2)
	case '0', '1', '2', '3', '4', '5', '6', '7':
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
			if i+2 < len(body) && body[i+2] == '{' {
				j := i + 3
				for j < len(body) && body[j] != '}' {
					j++
				}
				if j < len(body) {
					return j - i
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
