package parser

import (
	"fmt"
	"strconv"
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
	tkFString
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
	case tkFString:
		return "fstring"
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
	kind     tokKind
	val      string
	pos      Pos
	fpayload *fstringPayload
}

// fstringPayload carries the parsed structure of an f-string or
// t-string literal. The lexer slices the body into alternating text
// and interpolation segments; the parser then walks the segments to
// build JoinedStr / TemplateStr nodes.
type fstringPayload struct {
	template bool
	segments []fstringSegment
}

type fstringSegment struct {
	isInterp    bool
	text        string          // decoded text (for !isInterp)
	exprSrc     string          // source of the expression (for isInterp)
	convert     int             // -1, 'r', 's', or 'a'
	spec        *fstringPayload // optional format spec, recursively parsed
	pos         Pos             // position of the segment start
	selfDocText string          // PEP 701: raw source including '=' when self-doc
}

// parseSelfDoc detects a PEP 701 self-documenting '=' at the end of raw.
// Returns (exprPart, selfDocText, true) when found; (TrimSpace(raw), "", false)
// otherwise. selfDocText is the full raw content including trailing whitespace.
func parseSelfDoc(raw string) (exprPart, selfDocText string, ok bool) {
	stripped := strings.TrimRight(raw, " \t\r\n\f\v")
	if len(stripped) == 0 || stripped[len(stripped)-1] != '=' {
		return strings.TrimSpace(raw), "", false
	}
	before := stripped[:len(stripped)-1]
	if len(before) == 0 {
		return strings.TrimSpace(raw), "", false
	}
	prev := before[len(before)-1]
	// Exclude compound-assignment operators that end with '='.
	if prev == '=' || prev == '!' || prev == '<' || prev == '>' ||
		prev == '+' || prev == '-' || prev == '*' || prev == '/' ||
		prev == '%' || prev == '&' || prev == '|' || prev == '^' ||
		prev == '@' || prev == ':' {
		return strings.TrimSpace(raw), "", false
	}
	return strings.TrimSpace(before), raw, true
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
	isF := strings.ContainsRune(low, 'f')
	isT := strings.ContainsRune(low, 't')
	if isF && isT {
		return token{}, fmt.Errorf("%d:%d: cannot combine 'f' and 't' string prefixes", start.Line, start.Col)
	}
	if isT && strings.ContainsRune(low, 'b') {
		return token{}, fmt.Errorf("%d:%d: cannot combine 'b' and 't' string prefixes", start.Line, start.Col)
	}
	if isF || isT {
		return s.scanFTString(start, quote, low, isT)
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
	uPrefix := strings.ContainsRune(prefix, 'u') && !bytesPrefix
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
				if uPrefix {
					return token{kind: tkString, val: "u:" + val, pos: start}, nil
				}
				return token{kind: tkString, val: val, pos: start}, nil
			}
		} else if c == quote {
			s.advance(1)
			val := b.String()
			if bytesPrefix {
				return token{kind: tkString, val: "b:" + val, pos: start}, nil
			}
			if uPrefix {
				return token{kind: tkString, val: "u:" + val, pos: start}, nil
			}
			return token{kind: tkString, val: val, pos: start}, nil
		}
		if !raw && c == '\\' && s.off+1 < len(s.src) {
			esc := s.src[s.off+1]
			switch esc {
			case 'n':
				b.WriteByte('\n')
				s.advance(2)
			case 't':
				b.WriteByte('\t')
				s.advance(2)
			case 'r':
				b.WriteByte('\r')
				s.advance(2)
			case '\\':
				b.WriteByte('\\')
				s.advance(2)
			case '\'':
				b.WriteByte('\'')
				s.advance(2)
			case '"':
				b.WriteByte('"')
				s.advance(2)
			case 'a':
				b.WriteByte('\a')
				s.advance(2)
			case 'b':
				b.WriteByte('\b')
				s.advance(2)
			case 'f':
				b.WriteByte('\f')
				s.advance(2)
			case 'v':
				b.WriteByte('\v')
				s.advance(2)
			case '\n':
				// line continuation inside string: skip
				s.advance(2)
			case 'x':
				// \xHH — exactly 2 hex digits
				if s.off+3 < len(s.src) && isHexByte(s.src[s.off+2]) && isHexByte(s.src[s.off+3]) {
					hi := unhexByte(s.src[s.off+2])
					lo := unhexByte(s.src[s.off+3])
					val := rune(hi<<4 | lo)
					if bytesPrefix {
						b.WriteByte(byte(val))
					} else {
						b.WriteRune(val)
					}
					s.advance(4)
				} else {
					b.WriteByte('\\')
					b.WriteByte(esc)
					s.advance(2)
				}
			case '0', '1', '2', '3', '4', '5', '6', '7':
				// Octal escape: 1 to 3 octal digits
				octal := int(esc - '0')
				consumed := 2
				for i := 1; i < 3; i++ {
					if s.off+consumed < len(s.src) {
						d := s.src[s.off+consumed]
						if d >= '0' && d <= '7' {
							octal = octal*8 + int(d-'0')
							consumed++
							continue
						}
					}
					break
				}
				if bytesPrefix {
					b.WriteByte(byte(octal))
				} else {
					b.WriteRune(rune(octal))
				}
				s.advance(consumed)
			case 'u':
				// \uHHHH — 4 hex digits, strings only
				if !bytesPrefix && s.off+5 < len(s.src) &&
					isHexByte(s.src[s.off+2]) && isHexByte(s.src[s.off+3]) &&
					isHexByte(s.src[s.off+4]) && isHexByte(s.src[s.off+5]) {
					v, _ := strconv.ParseInt(string(s.src[s.off+2:s.off+6]), 16, 32)
					b.WriteRune(rune(v))
					s.advance(6)
				} else {
					b.WriteByte('\\')
					b.WriteByte(esc)
					s.advance(2)
				}
			case 'U':
				// \UHHHHHHHH — 8 hex digits, strings only
				if !bytesPrefix && s.off+9 < len(s.src) &&
					isHexByte(s.src[s.off+2]) && isHexByte(s.src[s.off+3]) &&
					isHexByte(s.src[s.off+4]) && isHexByte(s.src[s.off+5]) &&
					isHexByte(s.src[s.off+6]) && isHexByte(s.src[s.off+7]) &&
					isHexByte(s.src[s.off+8]) && isHexByte(s.src[s.off+9]) {
					v, _ := strconv.ParseInt(string(s.src[s.off+2:s.off+10]), 16, 32)
					b.WriteRune(rune(v))
					s.advance(10)
				} else {
					b.WriteByte('\\')
					b.WriteByte(esc)
					s.advance(2)
				}
			case 'N':
				// \N{name} — unicode name; leave as-is (rare in practice)
				b.WriteByte('\\')
				b.WriteByte(esc)
				s.advance(2)
			default:
				b.WriteByte('\\')
				b.WriteByte(esc)
				s.advance(2)
			}
			continue
		}
		b.WriteByte(c)
		s.advance(1)
	}
	return token{}, fmt.Errorf("%d:%d: unterminated string literal", start.Line, start.Col)
}

// scanFTString scans the body of an f-string or t-string into a
// payload of alternating text + interpolation segments. Format
// specs are themselves f-string bodies and are scanned recursively
// via scanFTBody. The scanner is positioned at the opening quote
// when called.
func (s *scanner) scanFTString(start Pos, quote byte, low string, isT bool) (token, error) {
	triple := s.peekByte(0) == quote && s.peekByte(1) == quote && s.peekByte(2) == quote
	if triple {
		s.advance(3)
	} else {
		s.advance(1)
	}
	raw := strings.ContainsRune(low, 'r')
	segs, err := s.scanFTBody(start, quote, raw, triple, false)
	if err != nil {
		return token{}, err
	}
	return token{
		kind: tkFString,
		pos:  start,
		fpayload: &fstringPayload{
			template: isT,
			segments: segs,
		},
	}, nil
}

// scanFTBody walks the body of an f/t-string up to a terminating
// quote (or, when inSpec is true, an unmatched `}` or `:`). The
// returned segments interleave text and interpolation pieces. When
// inSpec is true, the closing brace is left unconsumed for the
// caller to handle.
func (s *scanner) scanFTBody(start Pos, quote byte, raw, triple, inSpec bool) ([]fstringSegment, error) {
	var segs []fstringSegment
	var b strings.Builder
	textPos := s.pos()
	flush := func() {
		if b.Len() > 0 {
			segs = append(segs, fstringSegment{text: b.String(), pos: textPos})
			b.Reset()
		}
	}
	for s.off < len(s.src) {
		c := s.src[s.off]
		if !inSpec {
			if triple {
				if c == quote && s.peekByte(1) == quote && s.peekByte(2) == quote {
					s.advance(3)
					flush()
					return segs, nil
				}
			} else if c == quote {
				s.advance(1)
				flush()
				return segs, nil
			}
		} else {
			// Inside a format spec we stop at an unescaped `}` (the
			// end of the enclosing interpolation). `{` opens a nested
			// interpolation, handled below.
			if c == '}' {
				flush()
				return segs, nil
			}
		}
		if c == '{' {
			if s.peekByte(1) == '{' {
				b.WriteByte('{')
				s.advance(2)
				continue
			}
			flush()
			interpPos := s.pos()
			s.advance(1) // consume {
			seg, err := s.scanFTInterp(interpPos, quote, raw, triple)
			if err != nil {
				return nil, err
			}
			segs = append(segs, seg)
			textPos = s.pos()
			continue
		}
		if c == '}' {
			if s.peekByte(1) == '}' {
				b.WriteByte('}')
				s.advance(2)
				continue
			}
			return nil, fmt.Errorf("%d:%d: single '}' is not allowed", s.line, s.col)
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
				// line continuation
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
	if inSpec {
		flush()
		return segs, nil
	}
	return nil, fmt.Errorf("%d:%d: unterminated f-string literal", start.Line, start.Col)
}

// scanFTInterp scans one `{...}` interpolation. The opening `{` has
// already been consumed; on return the scanner sits past the closing
// `}`. The expression source is captured verbatim and parsed lazily
// in parseAtom — that keeps the lexer free of expression-grammar
// knowledge.
func (s *scanner) scanFTInterp(interpPos Pos, outerQuote byte, outerRaw, outerTriple bool) (fstringSegment, error) {
	exprStart := s.off
	depth := 0
	// PEP 701: track nested string literals so quotes inside the
	// expression can match the outer string.
	for s.off < len(s.src) {
		c := s.src[s.off]
		switch {
		case c == '{' || c == '(' || c == '[':
			depth++
			s.advance(1)
		case c == ')' || c == ']':
			if depth == 0 {
				return fstringSegment{}, fmt.Errorf("%d:%d: unexpected '%c' in f-string expression",
					s.line, s.col, c)
			}
			depth--
			s.advance(1)
		case c == '}':
			if depth > 0 {
				depth--
				s.advance(1)
				continue
			}
			// End of expression part.
			rawExpr := s.src[exprStart:s.off]
			s.advance(1) // consume }
			exprPart, selfDoc, _ := parseSelfDoc(rawExpr)
			return fstringSegment{
				isInterp:    true,
				exprSrc:     exprPart,
				convert:     -1,
				pos:         interpPos,
				selfDocText: selfDoc,
			}, nil
		case c == '!' && depth == 0 && s.peekByte(1) != '=':
			// Conversion suffix.
			rawExpr := s.src[exprStart:s.off]
			exprPart, selfDoc, _ := parseSelfDoc(rawExpr)
			s.advance(1)
			if s.off >= len(s.src) {
				return fstringSegment{}, fmt.Errorf("%d:%d: expected conversion char", s.line, s.col)
			}
			convCh := s.src[s.off]
			if convCh != 'r' && convCh != 's' && convCh != 'a' {
				return fstringSegment{}, fmt.Errorf("%d:%d: invalid conversion '%c'", s.line, s.col, convCh)
			}
			s.advance(1)
			seg := fstringSegment{
				isInterp:    true,
				exprSrc:     exprPart,
				convert:     int(convCh),
				pos:         interpPos,
				selfDocText: selfDoc,
			}
			// Optional format spec follows.
			if s.off < len(s.src) && s.src[s.off] == ':' {
				s.advance(1)
				specSegs, err := s.scanFTBody(interpPos, outerQuote, outerRaw, outerTriple, true)
				if err != nil {
					return fstringSegment{}, err
				}
				seg.spec = &fstringPayload{segments: specSegs}
			}
			if s.off >= len(s.src) || s.src[s.off] != '}' {
				return fstringSegment{}, fmt.Errorf("%d:%d: expected '}' to close interpolation", s.line, s.col)
			}
			s.advance(1)
			return seg, nil
		case c == ':' && depth == 0:
			rawExpr := s.src[exprStart:s.off]
			exprPart, selfDoc, _ := parseSelfDoc(rawExpr)
			s.advance(1)
			specSegs, err := s.scanFTBody(interpPos, outerQuote, outerRaw, outerTriple, true)
			if err != nil {
				return fstringSegment{}, err
			}
			seg := fstringSegment{
				isInterp:    true,
				exprSrc:     exprPart,
				convert:     -1,
				pos:         interpPos,
				spec:        &fstringPayload{segments: specSegs},
				selfDocText: selfDoc,
			}
			if s.off >= len(s.src) || s.src[s.off] != '}' {
				return fstringSegment{}, fmt.Errorf("%d:%d: expected '}' to close interpolation", s.line, s.col)
			}
			s.advance(1)
			return seg, nil
		case c == '\'' || c == '"':
			// Skip a nested string literal so its braces don't confuse us.
			if err := s.skipNestedString(); err != nil {
				return fstringSegment{}, err
			}
		case c == '#':
			return fstringSegment{}, fmt.Errorf("%d:%d: comments not allowed in f-string expression",
				s.line, s.col)
		default:
			s.advance(1)
		}
	}
	return fstringSegment{}, fmt.Errorf("%d:%d: unterminated f-string interpolation", s.line, s.col)
}

// skipNestedString advances past a nested string literal inside an
// f-string interpolation. Handles single and triple quotes, and the
// PEP 701 case where the inner quote may match the outer.
func (s *scanner) skipNestedString() error {
	q := s.src[s.off]
	startLine, startCol := s.line, s.col
	triple := s.peekByte(1) == q && s.peekByte(2) == q
	if triple {
		s.advance(3)
		for s.off < len(s.src) {
			if s.src[s.off] == q && s.peekByte(1) == q && s.peekByte(2) == q {
				s.advance(3)
				return nil
			}
			if s.src[s.off] == '\\' && s.off+1 < len(s.src) {
				s.advance(2)
				continue
			}
			s.advance(1)
		}
		return fmt.Errorf("%d:%d: unterminated triple-quoted string in f-string", startLine, startCol)
	}
	s.advance(1)
	for s.off < len(s.src) {
		c := s.src[s.off]
		if c == q {
			s.advance(1)
			return nil
		}
		if c == '\\' && s.off+1 < len(s.src) {
			s.advance(2)
			continue
		}
		if c == '\n' {
			return fmt.Errorf("%d:%d: unterminated string in f-string", startLine, startCol)
		}
		s.advance(1)
	}
	return fmt.Errorf("%d:%d: unterminated string in f-string", startLine, startCol)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
func isHexByte(c byte) bool { return isHexDigit(c) }
func unhexByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}
func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentPart(r rune) bool {
	if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	// UAX #31 Other_ID_Continue ranges not covered by IsLetter/IsDigit.
	return isOtherIDContinue(r)
}

// isOtherIDContinue reports whether r is in the Unicode Other_ID_Continue
// property. Only the ranges relevant to Python identifiers in practice are
// listed; the fixture that triggered this is U+E0100..U+E01EF (variation
// selectors supplement, used as tag characters in some East Asian encodings).
func isOtherIDContinue(r rune) bool {
	switch {
	case r == 0x00B7:
		return true // MIDDLE DOT
	case r == 0x0387:
		return true // GREEK ANO TELEIA
	case r >= 0x1369 && r <= 0x1371:
		return true // ETHIOPIC DIGIT ONE..NINE
	case r == 0x19DA:
		return true // NEW TAI LUE DIGIT ONE
	case r >= 0xE0100 && r <= 0xE01EF:
		return true // VARIATION SELECTORS SUPPLEMENT (tag chars)
	}
	return false
}
