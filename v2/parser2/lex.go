package parser2

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tokKind enumerates the lexical categories parser2 understands today.
// Anything outside this set is rejected as out-of-scope so failures
// surface loudly rather than silently dropping into garbage parses.
type tokKind int

const (
	tkEOF tokKind = iota
	tkInt
	tkFloat
	tkString
	tkName
	tkPlus
	tkMinus
	tkStar
	tkSlash
	tkTilde
	tkLParen
	tkRParen
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
	case tkSlash:
		return "/"
	case tkTilde:
		return "~"
	case tkLParen:
		return "("
	case tkRParen:
		return ")"
	}
	return fmt.Sprintf("tok(%d)", int(k))
}

type token struct {
	kind tokKind
	val  string
	pos  Pos
}

// scanner is a tiny single-pass tokenizer. It tracks line/col so
// errors point at the right spot. Reused buffers and per-call
// allocations are kept to a minimum because the bench depends on it.
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

func (s *scanner) skipSpace() {
	for s.off < len(s.src) {
		c := s.src[s.off]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			s.advance(1)
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
	switch {
	case c == '+':
		s.advance(1)
		return token{kind: tkPlus, val: "+", pos: start}, nil
	case c == '-':
		s.advance(1)
		return token{kind: tkMinus, val: "-", pos: start}, nil
	case c == '*':
		s.advance(1)
		return token{kind: tkStar, val: "*", pos: start}, nil
	case c == '/':
		s.advance(1)
		return token{kind: tkSlash, val: "/", pos: start}, nil
	case c == '~':
		s.advance(1)
		return token{kind: tkTilde, val: "~", pos: start}, nil
	case c == '(':
		s.advance(1)
		return token{kind: tkLParen, val: "(", pos: start}, nil
	case c == ')':
		s.advance(1)
		return token{kind: tkRParen, val: ")", pos: start}, nil
	case c == '"' || c == '\'':
		return s.scanString(start, c)
	case isDigit(c) || (c == '.' && s.off+1 < len(s.src) && isDigit(s.src[s.off+1])):
		return s.scanNumber(start)
	case isIdentStart(rune(c)) || c >= 0x80:
		return s.scanName(start)
	}
	return token{}, fmt.Errorf("%d:%d: unexpected character %q", start.Line, start.Col, c)
}

func (s *scanner) scanNumber(start Pos) (token, error) {
	begin := s.off
	isFloat := false
	for s.off < len(s.src) && isDigit(s.src[s.off]) {
		s.advance(1)
	}
	if s.off < len(s.src) && s.src[s.off] == '.' {
		isFloat = true
		s.advance(1)
		for s.off < len(s.src) && isDigit(s.src[s.off]) {
			s.advance(1)
		}
	}
	if s.off < len(s.src) && (s.src[s.off] == 'e' || s.src[s.off] == 'E') {
		isFloat = true
		s.advance(1)
		if s.off < len(s.src) && (s.src[s.off] == '+' || s.src[s.off] == '-') {
			s.advance(1)
		}
		for s.off < len(s.src) && isDigit(s.src[s.off]) {
			s.advance(1)
		}
	}
	val := s.src[begin:s.off]
	kind := tkInt
	if isFloat {
		kind = tkFloat
	}
	return token{kind: kind, val: val, pos: start}, nil
}

func (s *scanner) scanString(start Pos, quote byte) (token, error) {
	s.advance(1)
	var b strings.Builder
	for s.off < len(s.src) {
		c := s.src[s.off]
		if c == quote {
			s.advance(1)
			return token{kind: tkString, val: b.String(), pos: start}, nil
		}
		if c == '\\' && s.off+1 < len(s.src) {
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

func (s *scanner) scanName(start Pos) (token, error) {
	begin := s.off
	for s.off < len(s.src) {
		r, size := utf8.DecodeRuneInString(s.src[s.off:])
		if !isIdentPart(r) {
			break
		}
		s.advance(size)
	}
	val := s.src[begin:s.off]
	return token{kind: tkName, val: val, pos: start}, nil
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
