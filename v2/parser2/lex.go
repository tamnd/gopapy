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
	tkSemi     // ;
	tkArrow    // ->

	// Augmented assignment operators (statement mode)
	tkPlusAssign       // +=
	tkMinusAssign      // -=
	tkStarAssign       // *=
	tkSlashAssign      // /=
	tkDoubleSlAssign   // //=
	tkPercentAssign    // %=
	tkDoubleStarAssign // **=
	tkAmpAssign        // &=
	tkPipeAssign       // |=
	tkCaretAssign      // ^=
	tkLShiftAssign     // <<=
	tkRShiftAssign     // >>=
	tkAtAssign         // @=

	// Statement-level synthetic tokens. Only emitted when the scanner
	// is in statement mode; ParseExpression never sees them.
	tkNewline // logical newline at statement boundary
	tkIndent  // leading whitespace grew vs previous line
	tkDedent  // leading whitespace shrank vs previous line
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
	case tkSemi:
		return ";"
	case tkArrow:
		return "->"
	case tkPlusAssign:
		return "+="
	case tkMinusAssign:
		return "-="
	case tkStarAssign:
		return "*="
	case tkSlashAssign:
		return "/="
	case tkDoubleSlAssign:
		return "//="
	case tkPercentAssign:
		return "%="
	case tkDoubleStarAssign:
		return "**="
	case tkAmpAssign:
		return "&="
	case tkPipeAssign:
		return "|="
	case tkCaretAssign:
		return "^="
	case tkLShiftAssign:
		return "<<="
	case tkRShiftAssign:
		return ">>="
	case tkAtAssign:
		return "@="
	case tkNewline:
		return "NEWLINE"
	case tkIndent:
		return "INDENT"
	case tkDedent:
		return "DEDENT"
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
//
// In expression mode (the default and what ParseExpression uses) the
// scanner treats `\n` as whitespace and never emits NEWLINE / INDENT /
// DEDENT tokens. In statement mode (set via stmtMode for ParseFile)
// it emits a NEWLINE at every logical line boundary, INDENT when the
// leading whitespace of the next logical line is greater than the
// indent stack top, DEDENT(s) when it falls below, and tracks bracket
// depth so newlines inside (), [], {} stay invisible per Python's
// implicit line-joining rule.
type scanner struct {
	src  string
	off  int
	line int
	col  int

	// Statement-mode state. Zero value is fine for expression mode.
	stmtMode        bool
	indentStack     []int // current open indent levels; always starts with 0
	bracketDepth    int   // (), [], {} nesting; >0 disables NL/IN/DE
	atLineStart     bool  // true at file start and after NEWLINE
	pendingDedents  int   // queued DEDENTs to emit
	eofNewlineDone  bool  // whether the trailing NEWLINE-before-EOF was emitted
	eofDedentsDone  bool  // whether the trailing DEDENT chain was emitted
	lastEmittedKind tokKind
}

func newScanner(src string) *scanner {
	return &scanner{src: src, line: 1, col: 0}
}

// newStmtScanner returns a scanner primed for statement-level tokens:
// NEWLINE, INDENT, DEDENT plus everything ParseExpression already
// understands. The indent stack starts at [0]; `atLineStart` starts
// true so the first non-blank line gets indent-checked against zero
// (rejecting any leading whitespace at top level).
func newStmtScanner(src string) *scanner {
	return &scanner{
		src:         src,
		line:        1,
		col:         0,
		stmtMode:    true,
		indentStack: []int{0},
		atLineStart: true,
	}
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

// skipSpace eats whitespace and comments. In expression mode it
// consumes `\n` like any other space; in statement mode `\n` outside
// brackets stays put so `next` can convert it into a NEWLINE token.
func (s *scanner) skipSpace() {
	for s.off < len(s.src) {
		c := s.src[s.off]
		if s.stmtMode && s.bracketDepth == 0 && c == '\n' {
			break
		}
		if c == ' ' || c == '\t' || c == '\r' {
			s.advance(1)
			continue
		}
		if c == '\n' {
			s.advance(1)
			continue
		}
		// Line continuation: backslash + newline is whitespace
		// regardless of mode.
		if c == '\\' && s.peekByte(1) == '\n' {
			s.advance(2)
			continue
		}
		// Python comments run to end-of-line; treat like whitespace.
		// In statement mode the trailing `\n` is left for next() so a
		// comment followed by a newline still produces a NEWLINE token.
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
	tok, err := s.nextInternal()
	if err == nil {
		s.lastEmittedKind = tok.kind
	}
	return tok, err
}

func (s *scanner) nextInternal() (token, error) {
	// Drain any queued DEDENTs first so the caller sees them in the
	// correct order. Each DEDENT pops one level off the indent stack.
	if s.pendingDedents > 0 {
		s.pendingDedents--
		s.indentStack = s.indentStack[:len(s.indentStack)-1]
		return token{kind: tkDedent, pos: s.pos()}, nil
	}

	// Statement-mode line-start handling. Runs before skipSpace so the
	// leading indent is measurable. Skips blank/comment-only lines
	// without emitting NEWLINE (CPython's NL semantic). Inside brackets
	// indentation is irrelevant, so the branch is gated on
	// bracketDepth == 0.
	if s.stmtMode && s.atLineStart && s.bracketDepth == 0 {
		for {
			indent := 0
			for s.off < len(s.src) {
				c := s.src[s.off]
				if c == ' ' {
					indent++
					s.advance(1)
					continue
				}
				if c == '\t' {
					// CPython's tab stop: round up to next multiple of 8.
					indent += 8 - (indent % 8)
					s.advance(1)
					continue
				}
				break
			}
			// Blank line or comment-only line: not a logical line, just
			// keep scanning. Don't touch the indent stack.
			if s.off >= len(s.src) {
				break
			}
			if s.src[s.off] == '\n' {
				s.advance(1)
				continue
			}
			if s.src[s.off] == '#' {
				for s.off < len(s.src) && s.src[s.off] != '\n' {
					s.advance(1)
				}
				continue
			}
			// Real content on this line. Compare indent against stack.
			s.atLineStart = false
			top := s.indentStack[len(s.indentStack)-1]
			if indent > top {
				s.indentStack = append(s.indentStack, indent)
				return token{kind: tkIndent, pos: s.pos()}, nil
			}
			if indent < top {
				// Count how many levels we need to drop without mutating
				// the stack. Pop one here and let the drain branch handle
				// the rest, popping one per emitted DEDENT.
				count := 0
				i := len(s.indentStack) - 1
				for i > 0 && s.indentStack[i] > indent {
					count++
					i--
				}
				if s.indentStack[i] != indent {
					return token{}, fmt.Errorf("%d:%d: unindent does not match any outer indentation level",
						s.line, s.col)
				}
				s.indentStack = s.indentStack[:len(s.indentStack)-1]
				s.pendingDedents = count - 1
				return token{kind: tkDedent, pos: s.pos()}, nil
			}
			break
		}
	}

	s.skipSpace()

	// Statement-mode logical newline.
	if s.stmtMode && s.bracketDepth == 0 && s.off < len(s.src) && s.src[s.off] == '\n' {
		pos := s.pos()
		s.advance(1)
		s.atLineStart = true
		// Suppress doubled NEWLINEs at the lexer level so the parser
		// doesn't have to. CPython's tokenizer also collapses these.
		if s.lastEmittedKind == tkNewline || s.lastEmittedKind == 0 {
			return s.nextInternal()
		}
		return token{kind: tkNewline, pos: pos}, nil
	}

	if s.off >= len(s.src) {
		// At EOF in statement mode, synthesize a trailing NEWLINE (if
		// the file didn't end with one) followed by enough DEDENTs to
		// drain the indent stack. Then EOF.
		if s.stmtMode && !s.eofNewlineDone {
			s.eofNewlineDone = true
			if s.lastEmittedKind != tkNewline && s.lastEmittedKind != 0 {
				return token{kind: tkNewline, pos: s.pos()}, nil
			}
		}
		if s.stmtMode && !s.eofDedentsDone {
			s.eofDedentsDone = true
			if len(s.indentStack) > 1 {
				s.pendingDedents = len(s.indentStack) - 2
				s.indentStack = s.indentStack[:len(s.indentStack)-1]
				return token{kind: tkDedent, pos: s.pos()}, nil
			}
		}
		return token{kind: tkEOF, pos: s.pos()}, nil
	}
	start := s.pos()
	c := s.src[s.off]

	// Multi-char operators must be checked before single-char fallbacks.
	switch c {
	case '*':
		if s.peekByte(1) == '*' {
			if s.peekByte(2) == '=' {
				s.advance(3)
				return token{kind: tkDoubleStarAssign, val: "**=", pos: start}, nil
			}
			s.advance(2)
			return token{kind: tkDoubleStar, val: "**", pos: start}, nil
		}
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkStarAssign, val: "*=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkStar, val: "*", pos: start}, nil
	case '/':
		if s.peekByte(1) == '/' {
			if s.peekByte(2) == '=' {
				s.advance(3)
				return token{kind: tkDoubleSlAssign, val: "//=", pos: start}, nil
			}
			s.advance(2)
			return token{kind: tkDoubleSl, val: "//", pos: start}, nil
		}
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkSlashAssign, val: "/=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkSlash, val: "/", pos: start}, nil
	case '<':
		if s.peekByte(1) == '<' {
			if s.peekByte(2) == '=' {
				s.advance(3)
				return token{kind: tkLShiftAssign, val: "<<=", pos: start}, nil
			}
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
			if s.peekByte(2) == '=' {
				s.advance(3)
				return token{kind: tkRShiftAssign, val: ">>=", pos: start}, nil
			}
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
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkPlusAssign, val: "+=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkPlus, val: "+", pos: start}, nil
	case '-':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkMinusAssign, val: "-=", pos: start}, nil
		}
		if s.peekByte(1) == '>' {
			s.advance(2)
			return token{kind: tkArrow, val: "->", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkMinus, val: "-", pos: start}, nil
	case '%':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkPercentAssign, val: "%=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkPercent, val: "%", pos: start}, nil
	case '@':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkAtAssign, val: "@=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkAt, val: "@", pos: start}, nil
	case '&':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkAmpAssign, val: "&=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkAmp, val: "&", pos: start}, nil
	case '|':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkPipeAssign, val: "|=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkPipe, val: "|", pos: start}, nil
	case '^':
		if s.peekByte(1) == '=' {
			s.advance(2)
			return token{kind: tkCaretAssign, val: "^=", pos: start}, nil
		}
		s.advance(1)
		return token{kind: tkCaret, val: "^", pos: start}, nil
	case '~':
		s.advance(1)
		return token{kind: tkTilde, val: "~", pos: start}, nil
	case '(':
		s.bracketDepth++
		s.advance(1)
		return token{kind: tkLParen, val: "(", pos: start}, nil
	case ')':
		if s.bracketDepth > 0 {
			s.bracketDepth--
		}
		s.advance(1)
		return token{kind: tkRParen, val: ")", pos: start}, nil
	case '[':
		s.bracketDepth++
		s.advance(1)
		return token{kind: tkLBrack, val: "[", pos: start}, nil
	case ']':
		if s.bracketDepth > 0 {
			s.bracketDepth--
		}
		s.advance(1)
		return token{kind: tkRBrack, val: "]", pos: start}, nil
	case '{':
		s.bracketDepth++
		s.advance(1)
		return token{kind: tkLBrace, val: "{", pos: start}, nil
	case '}':
		if s.bracketDepth > 0 {
			s.bracketDepth--
		}
		s.advance(1)
		return token{kind: tkRBrace, val: "}", pos: start}, nil
	case ',':
		s.advance(1)
		return token{kind: tkComma, val: ",", pos: start}, nil
	case ';':
		s.advance(1)
		return token{kind: tkSemi, val: ";", pos: start}, nil
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
		return token{}, fmt.Errorf("%d:%d: %s literals are not implemented in v0.1.30",
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
