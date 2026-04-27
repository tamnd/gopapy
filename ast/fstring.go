package ast

import (
	"fmt"
	"strings"

	"github.com/tamnd/gopapy/legacy/parser"
)

// emitFString folds an implicitly-concatenated run of string literals
// (where at least one is an f-string) into a JoinedStr. Plain literal
// parts join in as Constant; f-string parts have their bodies split on
// `{ ... }` interpolation chunks and their expression text reparsed.
//
// The implementation is deliberately small: it handles the common cases
// (literal text, `{expr}`, `{expr!conv}`, `{expr:spec}`, `{{` and `}}`
// escapes, debug `{expr=}`) but does not try to support nested f-strings,
// nested braces inside the expression, or recursive format-spec parsing.
// Those are tracked for a follow-up release; the corpus check flags
// everything we miss.
func emitFString(p Pos, parts []string) ExprNode {
	var values []ExprNode
	var lit strings.Builder
	flush := func() {
		if lit.Len() == 0 {
			return
		}
		values = append(values, &Constant{Pos: p, Value: ConstantValue{Kind: ConstantStr, Str: lit.String()}})
		lit.Reset()
	}
	for _, raw := range parts {
		body, isF := stripStringQuotes(raw)
		if !isF {
			lit.WriteString(decodeEscapes(body))
			continue
		}
		raw := isRawFString(getStringPrefix(raw))
		body = applyEscapes(body, raw)
		i := 0
		for i < len(body) {
			c := body[i]
			if c == '{' {
				if i+1 < len(body) && body[i+1] == '{' {
					lit.WriteByte('{')
					i += 2
					continue
				}
				flush()
				end, fv := scanInterpolation(p, body, i+1, raw)
				if fv != nil {
					values = append(values, fv)
				}
				i = end
				continue
			}
			if c == '}' {
				if i+1 < len(body) && body[i+1] == '}' {
					lit.WriteByte('}')
					i += 2
					continue
				}
				// Lone `}` is a syntax error in Python; skip silently.
				i++
				continue
			}
			lit.WriteByte(c)
			i++
		}
	}
	flush()
	return &JoinedStr{Pos: p, Values: values}
}

// scanInterpolation parses one `{ ... }` chunk starting at i (just past
// the `{`) and returns (end-index after `}`, FormattedValue node). The
// parser tracks paren/bracket/brace depth and skips over strings so an
// embedded `}` doesn't terminate prematurely.
func scanInterpolation(p Pos, body string, i int, raw bool) (int, ExprNode) {
	depth := 0
	exprStart := i
	exprEnd := -1
	convStart := -1
	specStart := -1
	for i < len(body) {
		c := body[i]
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']':
			depth--
		case '\'', '"':
			i = skipString(body, i)
			continue
		case '!':
			if depth == 0 && exprEnd < 0 && i+1 < len(body) && body[i+1] != '=' {
				exprEnd = i
				convStart = i + 1
			}
		case ':':
			if depth == 0 && specStart < 0 {
				if exprEnd < 0 {
					exprEnd = i
				}
				specStart = i + 1
			}
		case '}':
			if depth > 0 {
				depth--
				break
			}
			if exprEnd < 0 {
				exprEnd = i
			}
			fv := buildFormattedValue(p, body[exprStart:exprEnd], convText(body, convStart, specStart, i), specText(body, specStart, i), raw)
			return i + 1, fv
		}
		i++
	}
	// Unterminated; bail with what we have.
	return len(body), nil
}

func convText(body string, convStart, specStart, end int) string {
	if convStart < 0 {
		return ""
	}
	stop := end
	if specStart >= 0 {
		stop = specStart - 1
	}
	return strings.TrimSpace(body[convStart:stop])
}

func specText(body string, specStart, end int) string {
	if specStart < 0 {
		return ""
	}
	return body[specStart:end]
}

func buildFormattedValue(p Pos, exprText, conv, spec string, raw bool) ExprNode {
	exprText = strings.TrimSpace(exprText)
	// Debug `{expr=}`: trailing `=` keeps the literal text and forces repr
	// when no conversion is given. Stripping the `=` keeps things simple.
	if strings.HasSuffix(exprText, "=") {
		exprText = strings.TrimSpace(exprText[:len(exprText)-1])
	}
	if exprText == "" {
		return nil
	}
	expr, err := parser.ParseExpression(exprText)
	if err != nil {
		// Surface as a Name so the dump still produces something rather
		// than panicking; downstream tooling will see the malformed text.
		return &Name{Pos: p, Id: fmt.Sprintf("<fstring-error:%v>", err), Ctx: &Load{}}
	}
	val := emitExpr(expr)
	fv := &FormattedValue{Pos: p, Value: val, Conversion: conversionOrd(conv)}
	if spec != "" {
		// The spec text comes from inside the outer braces, so applyEscapes
		// (which only touches depth-0 text) never decoded its escapes.
		// Treat the spec as a fresh f-string body and decode now.
		fv.FormatSpec = &JoinedStr{Pos: p, Values: parseFStringBody(p, applyEscapes(spec, raw), raw)}
	}
	return fv
}

// parseFStringBody splits a body string into the JoinedStr value list:
// runs of literal text become Constant, `{ ... }` chunks become
// FormattedValue. Used for both top-level f-strings and recursively for
// the format-spec inside `{x:>{width}}`.
func parseFStringBody(p Pos, body string, raw bool) []ExprNode {
	var values []ExprNode
	var lit strings.Builder
	flush := func() {
		if lit.Len() == 0 {
			return
		}
		values = append(values, &Constant{Pos: p, Value: ConstantValue{Kind: ConstantStr, Str: lit.String()}})
		lit.Reset()
	}
	i := 0
	for i < len(body) {
		c := body[i]
		if c == '{' {
			if i+1 < len(body) && body[i+1] == '{' {
				lit.WriteByte('{')
				i += 2
				continue
			}
			flush()
			end, fv := scanInterpolation(p, body, i+1, raw)
			if fv != nil {
				values = append(values, fv)
			}
			i = end
			continue
		}
		if c == '}' && i+1 < len(body) && body[i+1] == '}' {
			lit.WriteByte('}')
			i += 2
			continue
		}
		lit.WriteByte(c)
		i++
	}
	flush()
	return values
}

func conversionOrd(conv string) int {
	if len(conv) == 0 {
		return -1
	}
	return int(conv[0])
}

// skipString advances past a string literal starting at i (which points
// at the opening quote). Triple quotes count, escape sequences are
// honored. Returns the index just past the closing quote.
func skipString(body string, i int) int {
	q := body[i]
	if i+2 < len(body) && body[i+1] == q && body[i+2] == q {
		j := i + 3
		for j+2 < len(body) {
			if body[j] == '\\' {
				j += 2
				continue
			}
			if body[j] == q && body[j+1] == q && body[j+2] == q {
				return j + 3
			}
			j++
		}
		return len(body)
	}
	j := i + 1
	for j < len(body) {
		if body[j] == '\\' {
			j += 2
			continue
		}
		if body[j] == q {
			return j + 1
		}
		j++
	}
	return len(body)
}

// stripStringQuotes returns (body, isFString) for a raw STRING token.
// Body is the text between the matching quotes; isFString reports
// whether the prefix contained `f` or `F`.
func stripStringQuotes(raw string) (string, bool) {
	prefix := getStringPrefix(raw)
	rest := raw[len(prefix):]
	if len(rest) >= 6 && (strings.HasPrefix(rest, `"""`) || strings.HasPrefix(rest, `'''`)) {
		body := rest[3 : len(rest)-3]
		return body, hasFPrefix(prefix)
	}
	if len(rest) >= 2 {
		return rest[1 : len(rest)-1], hasFPrefix(prefix)
	}
	return rest, hasFPrefix(prefix)
}

func getStringPrefix(raw string) string {
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\'' || raw[i] == '"' {
			return raw[:i]
		}
	}
	return raw
}

func hasFPrefix(prefix string) bool {
	for i := 0; i < len(prefix); i++ {
		c := prefix[i] | 0x20
		if c == 'f' || c == 't' {
			return true
		}
	}
	return false
}

func isRawFString(prefix string) bool {
	for i := 0; i < len(prefix); i++ {
		if prefix[i]|0x20 == 'r' {
			return true
		}
	}
	return false
}

// applyEscapes expands escape sequences in an f-string body, except inside
// `{ ... }` interpolations where the expression text must stay verbatim
// (CPython's f-string parser handles those specially). Raw f-strings
// preserve backslashes.
func applyEscapes(body string, raw bool) string {
	if raw {
		return body
	}
	var b strings.Builder
	i := 0
	depth := 0
	for i < len(body) {
		c := body[i]
		if c == '{' {
			if i+1 < len(body) && body[i+1] == '{' {
				b.WriteString("{{")
				i += 2
				continue
			}
			depth++
			b.WriteByte(c)
			i++
			continue
		}
		if c == '}' && depth > 0 {
			depth--
			b.WriteByte(c)
			i++
			continue
		}
		if depth > 0 {
			b.WriteByte(c)
			i++
			continue
		}
		if c == '\\' && i+1 < len(body) {
			n := escapeRunLen(body, i)
			b.WriteString(decodeEscapes(body[i : i+n]))
			i += n
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// escapeRunLen returns the number of bytes consumed by an escape sequence
// starting at body[i] (which must point at a backslash with at least one
// following byte). The caller passes that slice to decodeEscapes; without
// the right length, multi-byte escapes like \xHH, \uHHHH, \UHHHHHHHH or
// 1-3 digit octals get truncated and emitted verbatim.
func escapeRunLen(body string, i int) int {
	c := body[i+1]
	switch c {
	case 'x':
		if i+3 < len(body) && isHex(body[i+2]) && isHex(body[i+3]) {
			return 4
		}
	case 'u':
		if i+5 < len(body) && allHex(body[i+2:i+6]) {
			return 6
		}
	case 'U':
		if i+9 < len(body) && allHex(body[i+2:i+10]) {
			return 10
		}
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n := 2
		if i+2 < len(body) && isOctal(body[i+2]) {
			n++
			if i+3 < len(body) && isOctal(body[i+3]) {
				n++
			}
		}
		return n
	}
	return 2
}
