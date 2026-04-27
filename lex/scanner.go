package lex

import (
	"unicode"
	"unicode/utf8"
)

// Scanner produces physical tokens (no INDENT/DEDENT/NEWLINE injection).
// It is the byte-level reader; lex/indent.go wraps it to add the synthetic
// tokens Python's grammar references.
type Scanner struct {
	src      []byte
	pos      int // byte offset
	line     int // 1-indexed
	col      int // 0-indexed UTF-8 byte column
	filename string
}

// NewScanner returns a Scanner over src. filename is used only for error
// messages.
//
// A leading UTF-8 BOM (EF BB BF) is silently skipped; CPython's tokenizer
// does the same. Carriage returns are normalised to newlines so CRLF and
// bare-CR line endings tokenise the same as LF.
func NewScanner(src []byte, filename string) *Scanner {
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		src = src[3:]
	}
	if hasCarriageReturn(src) {
		src = normalizeNewlines(src)
	}
	return &Scanner{src: src, line: 1, filename: filename}
}

func hasCarriageReturn(src []byte) bool {
	for _, b := range src {
		if b == '\r' {
			return true
		}
	}
	return false
}

// normalizeNewlines converts CRLF and bare CR to LF, matching CPython's
// universal-newlines behaviour for source files.
func normalizeNewlines(src []byte) []byte {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == '\r' {
			out = append(out, '\n')
			if i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			continue
		}
		out = append(out, c)
	}
	return out
}

// Position reports the current scanner position.
func (s *Scanner) Position() Position {
	return Position{Filename: s.filename, Offset: s.pos, Line: s.line, Col: s.col}
}

// Done is true at EOF.
func (s *Scanner) Done() bool { return s.pos >= len(s.src) }

// peek returns the byte at offset pos+i without advancing. Returns 0 past EOF.
func (s *Scanner) peek(i int) byte {
	if s.pos+i >= len(s.src) {
		return 0
	}
	return s.src[s.pos+i]
}

// advance moves forward by n bytes, updating line/col. n must be valid UTF-8
// boundaries — callers that consume runes pass the rune width.
func (s *Scanner) advance(n int) {
	end := min(s.pos+n, len(s.src))
	for s.pos < end {
		c := s.src[s.pos]
		if c == '\n' {
			s.line++
			s.col = 0
		} else {
			s.col++
		}
		s.pos++
	}
}

// Scan returns the next physical token, or EOF when the input is exhausted.
// The synthetic NEWLINE / INDENT / DEDENT tokens are added by the indent
// wrapper, not by Scan.
func (s *Scanner) Scan() (Token, error) {
	for {
		if s.Done() {
			return s.tok(EOF, ""), nil
		}
		// skip horizontal whitespace (including form feed and vertical tab,
		// which CPython's tokenizer treats as whitespace)
		c := s.peek(0)
		if c == ' ' || c == '\t' || c == '\f' || c == '\v' {
			s.advance(1)
			continue
		}
		// line continuation: backslash at EOL
		if c == '\\' && s.peek(1) == '\n' {
			s.advance(2)
			continue
		}
		break
	}

	start := s.Position()
	c := s.peek(0)

	switch {
	case c == '\n':
		s.advance(1)
		return Token{Kind: NEWLINE, Value: "\n", Pos: start, End: s.Position()}, nil
	case c == '#':
		// comment to end of line; preserved as a COMMENT token (the indent
		// wrapper drops these unless they're TYPE_COMMENT).
		startOff := s.pos
		for !s.Done() && s.peek(0) != '\n' {
			s.advance(1)
		}
		text := string(s.src[startOff:s.pos])
		kind := COMMENT
		if isTypeComment(text) {
			kind = TYPE_COMMENT
		}
		return Token{Kind: kind, Value: text, Pos: start, End: s.Position()}, nil
	case c == '0' && (s.peek(1) == 'x' || s.peek(1) == 'X' || s.peek(1) == 'o' || s.peek(1) == 'O' || s.peek(1) == 'b' || s.peek(1) == 'B'):
		return s.scanRadix(start)
	case isDigit(c):
		return s.scanNumber(start)
	case c == '.' && isDigit(s.peek(1)):
		return s.scanNumber(start)
	case c < 0x80 && isIdentStart(rune(c)):
		return s.scanNameOrString(start)
	case c >= 0x80:
		// Multi-byte UTF-8 lead. Decode and check whether the rune is a
		// legal identifier start; otherwise reject explicitly so we don't
		// loop forever on a character we can't classify.
		r, n := utf8.DecodeRune(s.src[s.pos:])
		if isIdentStart(r) {
			return s.scanNameOrString(start)
		}
		s.advance(n)
		return Token{}, &Error{Pos: start, Msg: "unexpected character " + quoteRune(r)}
	case c == '"' || c == '\'':
		return s.scanString(start, "")
	}
	return s.scanOp(start)
}

func (s *Scanner) tok(k Kind, v string) Token {
	return Token{Kind: k, Value: v, Pos: s.Position(), End: s.Position()}
}

// scanNameOrString handles identifiers and prefixed string literals
// (r, b, u, f, t, rb, br, rf, fr, ft, tf, ...). The prefix matters because
// it shifts the lexer mode for f-strings and t-strings.
func (s *Scanner) scanNameOrString(start Position) (Token, error) {
	startOff := s.pos
	for !s.Done() {
		r, n := utf8.DecodeRune(s.src[s.pos:])
		if !isIdentPart(r) {
			break
		}
		s.advance(n)
	}
	name := string(s.src[startOff:s.pos])

	// Possible string prefix: identifier directly followed by " or '.
	if s.peek(0) == '"' || s.peek(0) == '\'' {
		if isStringPrefix(name) {
			return s.scanString(start, name)
		}
	}
	return Token{Kind: NAME, Value: name, Pos: start, End: s.Position()}, nil
}

// scanOp recognises every Python operator/delimiter, longest match first.
func (s *Scanner) scanOp(start Position) (Token, error) {
	for _, op := range operatorTable {
		if s.match(op.lit) {
			s.advance(len(op.lit))
			return Token{Kind: op.kind, Value: op.lit, Pos: start, End: s.Position()}, nil
		}
	}
	return Token{}, &Error{
		Pos: start,
		Msg: "unexpected character " + quoteRune(rune(s.peek(0))),
	}
}

// match reports whether the upcoming bytes equal lit.
func (s *Scanner) match(lit string) bool {
	if s.pos+len(lit) > len(s.src) {
		return false
	}
	for i := 0; i < len(lit); i++ {
		if s.src[s.pos+i] != lit[i] {
			return false
		}
	}
	return true
}

type opRow struct {
	lit  string
	kind Kind
}

// operatorTable lists operators sorted longest-first so prefix matches don't
// shadow the longer forms (e.g. `**=` before `**` before `*`).
var operatorTable = []opRow{
	{"**=", DOUBLESTAREQ}, {"//=", DOUBLESLEQ}, {"<<=", LSHIFTEQ}, {">>=", RSHIFTEQ}, {"...", ELLIPSIS},
	{"**", DOUBLESTAR}, {"//", DOUBLESLASH}, {"<<", LSHIFT}, {">>", RSHIFT},
	{"<=", LE}, {">=", GE}, {"==", EQEQ}, {"!=", NE}, {"->", ARROW}, {":=", WALRUS},
	{"+=", PLUSEQ}, {"-=", MINUSEQ}, {"*=", STAREQ}, {"/=", SLASHEQ},
	{"%=", PERCENTEQ}, {"@=", ATEQ}, {"&=", AMPEQ}, {"|=", PIPEEQ}, {"^=", CARETEQ},
	{"+", PLUS}, {"-", MINUS}, {"*", STAR}, {"/", SLASH}, {"%", PERCENT}, {"@", AT},
	{"&", AMP}, {"|", PIPE}, {"^", CARET}, {"~", TILDE},
	{"<", LT}, {">", GT}, {"=", EQ},
	{"(", LPAREN}, {")", RPAREN}, {"[", LBRACK}, {"]", RBRACK}, {"{", LBRACE}, {"}", RBRACE},
	{",", COMMA}, {":", COLON}, {";", SEMI}, {".", DOT},
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentPart(r rune) bool {
	if isIdentStart(r) || unicode.IsDigit(r) {
		return true
	}
	// Combining marks (Mn, Mc) and connector punctuation (Pc) are part
	// of an identifier per UAX #31. The tag-character block
	// (U+E0100..U+E01EF) is listed in Other_ID_Continue and is allowed
	// by Python in identifiers.
	if unicode.IsMark(r) || unicode.Is(unicode.Pc, r) {
		return true
	}
	if r >= 0xE0100 && r <= 0xE01EF {
		return true
	}
	return false
}

func isStringPrefix(s string) bool {
	if len(s) > 2 {
		return false
	}
	low := lower(s)
	switch low {
	case "r", "u", "b", "f", "t",
		"rb", "br", "rf", "fr", "rt", "tr",
		"bf", "fb", // not really valid Python — caller will reject if combined wrong
		"bt", "tb":
		return true
	}
	return false
}

func lower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

func quoteRune(r rune) string {
	if r == 0 {
		return "<EOF>"
	}
	return "'" + string(r) + "'"
}

func isTypeComment(text string) bool {
	// `# type: ...` with optional whitespace.
	const prefix = "# type:"
	if len(text) < len(prefix) || text[0] != '#' {
		return false
	}
	i := 1
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	if i+5 > len(text) {
		return false
	}
	return text[i:i+5] == "type:"
}
