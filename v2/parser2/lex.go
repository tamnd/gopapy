package parser2

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tokKind enumerates the lexical categories parser2 understands.
// Anything outside this set is rejected as out-of-scope so failures
// surface loudly rather than silently dropping into garbage parses.
type tokKind int

const (
	tkEOF tokKind = iota
	tkInt
	tkFloat
	tkString
	tkName

	// Arithmetic
	tkPlus       // +
	tkMinus      // -
	tkStar       // *
	tkDoubleStar // **
	tkSlash      // /
	tkDoubleSl   // //
	tkPercent    // %
	tkAt         // @

	// Bitwise + shift
	tkAmp    // &
	tkPipe   // |
	tkCaret  // ^
	tkTilde  // ~
	tkLShift // <<
	tkRShift // >>

	// Comparison
	tkLt    // <
	tkGt    // >
	tkLe    // <=
	tkGe    // >=
	tkEqEq  // ==
	tkNotEq // !=

	// Assignment-ish
	tkAssign // =  (only meaningful inside call kwargs / lambda defaults)
	tkWalrus // :=

	// Brackets and delimiters
	tkLParen   // (
	tkRParen   // )
	tkLBrack   // [
	tkRBrack   // ]
	tkLBrace   // {
	tkRBrace   // }
	tkComma    // ,
	tkColon    // :
	tkDot      // .
	tkEllipsis // ...
)

func (k tokKind) String() string {
	switch k {
	case tkEOF:
		return "EOF"
	case tkInt:
		return "int"
	case tkFloat:
		return "float"
	case tkString:
		return "string"
	case tkName:
		return "name"
	case tkPlus:
		return "+"
	case tkMinus:
		return "-"
	case tkStar:
		return "*"
	case tkDoubleStar:
		return "**"
	case tkSlash:
		return "/"
	case tkDoubleSl:
		return "//"
	case tkPercent:
		return "%"
	case tkAt:
		return "@"
	case tkAmp:
		return "&"
	case tkPipe:
		return "|"
	case tkCaret:
		return "^"
	case tkTilde:
		return "~"
	case tkLShift:
		return "<<"
	case tkRShift:
		return ">>"
	case tkLt:
		return "<"
	case tkGt:
		return ">"
	case tkLe:
		return "<="
	case tkGe:
		return ">="
	case tkEqEq:
		return "=="
	case tkNotEq:
		return "!="
	case tkAssign:
		return "="
	case tkWalrus:
		return ":="
	case tkLParen:
		return "("
	case tkRParen:
		return ")"
	case tkLBrack:
		return "["
	case tkRBrack:
		return "]"
	case tkLBrace:
		return "{"
	case tkRBrace:
		return "}"
	case tkComma:
		return ","
	case tkColon:
		return ":"
	case tkDot:
		return "."
	case tkEllipsis:
		return "..."
	}
	return fmt.Sprintf("tok(%d)", int(k))
}

type token struct {
	kind tokKind
	val  string
	pos  Pos
}

// scanner is a single-pass tokenizer with peek-ahead for multi-char
// operators. Reused buffers and per-call allocations are kept to a
// minimum because the bench depends on it.
type scanner struct {
	src  string
	off  int
	line int
	col  int
}

func newScanner(src string) *scanner {
	return &scanner{src: src, line: 1, col: 0}
}

func (s *scanner) pos() Pos { return Pos{Line: s.line, Col: s.col} }

func (s *scanner) advance(n int) {
	for i := range n {
		if s.off+i >= len(s.src) {
			break
		}
		if s.src[s.off+i] == '\n' {
			s.line++
			s.col = 0
		} else {
			s.col++
		}
	}
	s.off += n
}

func (s *scanner) peekByte(n int) byte {
	if s.off+n >= len(s.src) {
		return 0
	}
	return s.src[s.off+n]
}

func (s *scanner) skipSpace() {
	for s.off < len(s.src) {
		c := s.src[s.off]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			s.advance(1)
			continue
		}
		// Line continuation: backslash + newline is whitespace inside
		// expressions.
		if c == '\\' && s.peekByte(1) == '\n' {
			s.advance(2)
			continue
		}
		// Python comments run to end-of-line; treat like whitespace
		// inside expressions.
		if c == '#' {
			for s.off < len(s.src) && s.src[s.off] != '\n' {
				s.advance(1)
			}
			continue
		}
		break
	}
}

func (s *scanner) next() (token, error) {
	s.skipSpace()
	if s.off >= len(s.src) {
		return token{kind: tkEOF, pos: s.pos()}, nil
	}
	start := s.pos()
	c := s.src[s.off]

	// Multi-char operators must be checked before single-char fallbacks.
	switch c {
	case '*':
		if s.peekByte(1) == '*' {
			s.advance(2)
			return token{kind: tkDoubleStar, val: "**", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkStar, val: "*", pos: start}, nil
	case '/':
		if s.peekByte(1) == '/' {
			s.advance(2)
			return token{kind: tkDoubleSl, val: "//", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkSlash, val: "/", pos: start}, nil
	case '<':
		if s.peekByte(1) == '<' {
			s.advance(2)
			return token{kind: tkLShift, val: "<<", pos: start}, nil
		}
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkLe, val: "<=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkLt, val: "<", pos: start}, nil
	case '>':
		if s.peekByte(1) == '>' {
			s.advance(2)
			return token{kind: tkRShift, val: ">>", pos: start}, nil
		}
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkGe, val: ">=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkGt, val: ">", pos: start}, nil
	case '=':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkEqEq, val: "==", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkAssign, val: "=", pos: start}, nil
	case '!':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkNotEq, val: "!=", pos: start}, nil
		}
		return token{}, fmt.Errorf("%d:%d: '!' is not a valid token",
			start.Line, start.Col)
	case ':':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkWalrus, val: ":=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkColon, val: ":", pos: start}, nil
	case '.':
		// `...` ellipsis literal beats single `.`
		if s.peekByte(1) == '.' && s.peekByte(2) == '.' {
			s.advance(3)
			return token{kind: tkEllipsis, val: "...", pos: start}, nil
		}
		// `.5` is a float literal.
		if isDigit(s.peekByte(1)) {
			return s.scanNumber(start)
		}
		s.advance(1)
		return token{kind: tkDot, val: ".", pos: start}, nil
	case '+':
		s.advance(1)
		return token{kind: tkPlus, val: "+", pos: start}, nil
	case '-':
		s.advance(1)
		return token{kind: tkMinus, val: "-", pos: start}, nil
	case '%':
		s.advance(1)
		return token{kind: tkPercent, val: "%", pos: start}, nil
	case '@':
		s.advance(1)
		return token{kind: tkAt, val: "@", pos: start}, nil
	case '&':
		s.advance(1)
		return token{kind: tkAmp, val: "&", pos: start}, nil
	case '|':
		s.advance(1)
		return token{kind: tkPipe, val: "|", pos: start}, nil
	case '^':
		s.advance(1)
		return token{kind: tkCaret, val: "^", pos: start}, nil
	case '~':
		s.advance(1)
		return token{kind: tkTilde, val: "~", pos: start}, nil
	case '(':
		s.advance(1)
		return token{kind: tkLParen, val: "(", pos: start}, nil
	case ')':
		s.advance(1)
		return token{kind: tkRParen, val: ")", pos: start}, nil
	case '[':
		s.advance(1)
		return token{kind: tkLBrack, val: "[", pos: start}, nil
	case ']':
		s.advance(1)
		return token{kind: tkRBrack, val: "]", pos: start}, nil
	case '{':
		s.advance(1)
		return token{kind: tkLBrace, val: "{", pos: start}, nil
	case '}':
		s.advance(1)
		return token{kind: tkRBrace, val: "}", pos: start}, nil
	case ',':
		s.advance(1)
		return token{kind: tkComma, val: ",", pos: start}, nil
	case '"', '\'':
		return s.scanString(start, c, "")
	}

	switch {
	case isDigit(c):
		return s.scanNumber(start)
	case isIdentStart(rune(c)) || c >= 0x80:
		return s.scanNameOrPrefixedString(start)
	}
	return token{}, fmt.Errorf("%d:%d: unexpected character %q",
		start.Line, start.Col, c)
}

// scanNumber handles int (decimal, hex, oct, bin), float (with
// optional fraction and exponent), and complex (j/J suffix) literals.
// Underscore separators are stripped from the value text for
// strconv.Parse* to consume.
func (s *scanner) scanNumber(start Pos) (token, error) {
	begin := s.off
	isFloat := false

	// Hex / oct / bin literals.
	if s.src[s.off] == '0' && s.off+1 < len(s.src) {
		next := s.src[s.off+1]
		if next == 'x' || next == 'X' {
			s.advance(2)
			for s.off < len(s.src) && (isHexDigit(s.src[s.off]) || s.src[s.off] == '_') {
				s.advance(1)
			}
			return token{kind: tkInt, val: s.src[begin:s.off], pos: start}, nil
		}
		if next == 'o' || next == 'O' {
			s.advance(2)
			for s.off < len(s.src) && ((s.src[s.off] >= '0' && s.src[s.off] <= '7') || s.src[s.off] == '_') {
				s.advance(1)
			}
			return token{kind: tkInt, val: s.src[begin:s.off], pos: start}, nil
		}
		if next == 'b' || next == 'B' {
			s.advance(2)
			for s.off < len(s.src) && (s.src[s.off] == '0' || s.src[s.off] == '1' || s.src[s.off] == '_') {
				s.advance(1)
			}
			return token{kind: tkInt, val: s.src[begin:s.off], pos: start}, nil
		}
	}

	// Decimal / float / complex.
	for s.off < len(s.src) && (isDigit(s.src[s.off]) || s.src[s.off] == '_') {
		s.advance(1)
	}
	if s.off < len(s.src) && s.src[s.off] == '.' {
		isFloat = true
		s.advance(1)
		for s.off < len(s.src) && (isDigit(s.src[s.off]) || s.src[s.off] == '_') {
			s.advance(1)
		}
	}
	if s.off < len(s.src) && (s.src[s.off] == 'e' || s.src[s.off] == 'E') {
		isFloat = true
		s.advance(1)
		if s.off < len(s.src) && (s.src[s.off] == '+' || s.src[s.off] == '-') {
			s.advance(1)
		}
		for s.off < len(s.src) && (isDigit(s.src[s.off]) || s.src[s.off] == '_') {
			s.advance(1)
		}
	}
	if s.off < len(s.src) && (s.src[s.off] == 'j' || s.src[s.off] == 'J') {
		s.advance(1)
		// Complex literal — represented as a float-kind Constant with
		// the trailing 'j' preserved in val.
		return token{kind: tkFloat, val: s.src[begin:s.off], pos: start}, nil
	}
	val := s.src[begin:s.off]
	kind := tkInt
	if isFloat {
		kind = tkFloat
	}
	return token{kind: kind, val: val, pos: start}, nil
}

// scanNameOrPrefixedString handles bare identifiers and string
// literals with prefixes like `b`, `r`, `rb`, `f`, `t`, etc. It
// peeks one rune past the identifier; if that rune is `"` or `'`,
// it commits to scanning a prefixed string literal.
func (s *scanner) scanNameOrPrefixedString(start Pos) (token, error) {
	begin := s.off
	for s.off < len(s.src) {
		r, size := utf8.DecodeRuneInString(s.src[s.off:])
		if !isIdentPart(r) {
			break
		}
		s.advance(size)
	}
	val := s.src[begin:s.off]
	// Prefixed string literal? Up to two-char prefixes: b, r, u, f, t,
	// br, rb, fr, rf, tr, rt.
	if s.off < len(s.src) && (s.src[s.off] == '"' || s.src[s.off] == '\'') {
		if isStringPrefix(val) {
			quote := s.src[s.off]
			return s.scanString(start, quote, val)
		}
	}
	return token{kind: tkName, val: val, pos: start}, nil
}

func isStringPrefix(p string) bool {
	if len(p) == 0 || len(p) > 2 {
		return false
	}
	low := strings.ToLower(p)
	switch low {
	case "b", "r", "u", "f", "t",
		"br", "rb", "fr", "rf", "tr", "rt", "bu", "ub":
		return true
	}
	return false
}

// scanString parses a quoted string literal and returns its decoded
// text in the token value. F-strings and t-strings are flagged as
// out-of-scope at this version. Triple-quoted strings are supported.
func (s *scanner) scanString(start Pos, quote byte, prefix string) (token, error) {
	low := strings.ToLower(prefix)
	if strings.ContainsAny(low, "ft") {
		kind := "f-string"
		if strings.ContainsRune(low, 't') {
			kind = "t-string"
		}
		return token{}, fmt.Errorf("%d:%d: %s literals are not implemented in v0.1.29",
			start.Line, start.Col, kind)
	}
	// The prefix bytes were already consumed by scanNameOrPrefixedString;
	// the scanner is positioned at the opening quote.
	// Triple quoted?
	triple := s.peekByte(0) == quote && s.peekByte(1) == quote && s.peekByte(2) == quote
	if triple {
		s.advance(3)
	} else {
		s.advance(1)
	}
	raw := strings.ContainsRune(low, 'r')
	bytesPrefix := strings.ContainsRune(low, 'b')
	var b strings.Builder
	for s.off < len(s.src) {
		c := s.src[s.off]
		if triple {
			if c == quote && s.peekByte(1) == quote && s.peekByte(2) == quote {
				s.advance(3)
				val := b.String()
				if bytesPrefix {
					return token{kind: tkString, val: "b:" + val, pos: start}, nil
				}
				return token{kind: tkString, val: val, pos: start}, nil
			}
		} else if c == quote {
			s.advance(1)
			val := b.String()
			if bytesPrefix {
				return token{kind: tkString, val: "b:" + val, pos: start}, nil
			}
			return token{kind: tkString, val: val, pos: start}, nil
		}
		if !raw && c == '\\' && s.off+1 < len(s.src) {
			esc := s.src[s.off+1]
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '\'':
				b.WriteByte('\'')
			case '"':
				b.WriteByte('"')
			case '0':
				b.WriteByte(0)
			case '\n':
				// line continuation inside string: skip
			default:
				b.WriteByte('\\')
				b.WriteByte(esc)
			}
			s.advance(2)
			continue
		}
		b.WriteByte(c)
		s.advance(1)
	}
	return token{}, fmt.Errorf("%d:%d: unterminated string literal", start.Line, start.Col)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
