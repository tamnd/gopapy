package lex

// String literals.
//
// This file handles the simple string forms: short and triple-quoted strings
// with the prefixes that don't introduce interpolation (r, b, u, rb, br, and
// any combination thereof). f-strings and t-strings need a state machine
// because their bodies can contain arbitrary expressions; that lives in
// lex/state.go and lands in PR2.

func (s *Scanner) scanString(start Position, prefix string) (Token, error) {
	if hasInterpolatedPrefix(prefix) {
		return s.scanInterpolatedString(start, prefix)
	}
	return s.scanFlatString(start, prefix)
}

// scanInterpolatedString reads an f-string or t-string body, tracking
// brace depth so that nested string literals inside `{ ... }`
// interpolations don't terminate the outer string. PEP 701 lets the
// inner string reuse the outer quote character: f"{"hello"}" is valid.
//
// The scanner still emits a single STRING token containing the full raw
// source. Splitting the body into literal chunks and interpolations is
// the emitter's job.
func (s *Scanner) scanInterpolatedString(start Position, prefix string) (Token, error) {
	startOff := s.pos
	quote := s.peek(0)
	triple := false
	if s.peek(1) == quote && s.peek(2) == quote {
		triple = true
		s.advance(3)
	} else {
		s.advance(1)
	}
	raw := isRawPrefix(prefix)
	depth := 0
	// stack tracks every opener at the current nesting level. Each
	// frame says (a) what kind of opener it is — only `{` may
	// participate in format-spec mode — and (b) whether we have already
	// entered the spec for that brace (after the first `:`). This lets
	// a recursive replacement field inside a spec
	// (`f'{x:{y}>10}'`) re-enter expression mode for the inner `{...}`
	// and then drop back to spec when its `}` closes.
	type frame struct {
		brace bool
		spec  bool
	}
	stack := []frame{}
	inSpec := false
	for {
		if s.Done() {
			return Token{}, &Error{Pos: start, Msg: "unterminated string literal"}
		}
		c := s.peek(0)
		if c == '\\' && !raw && !inSpec {
			s.advance(1)
			if !s.Done() {
				s.advance(1)
			}
			continue
		}
		if c == '\\' && raw && !inSpec {
			// In raw f-strings the backslash is preserved verbatim and
			// does not introduce a semantic escape. Lexically it still
			// pairs with the next byte so a `\"`, `\'`, or `\\` cannot
			// terminate the literal early. Single-byte advance otherwise
			// so a following `{{` or `}}` is still recognised as the
			// PEP 701 doubled-brace escape: fr'\{{' is the two-char
			// string `\{`.
			n := s.peek(1)
			if n == quote || n == '\\' {
				s.advance(2)
			} else {
				s.advance(1)
			}
			continue
		}
		if depth == 0 {
			if c == '{' {
				if s.peek(1) == '{' {
					s.advance(2)
					continue
				}
				depth = 1
				stack = append(stack, frame{brace: true})
				s.advance(1)
				continue
			}
			if c == '}' && s.peek(1) == '}' {
				s.advance(2)
				continue
			}
			if c == '\n' && !triple {
				return Token{}, &Error{Pos: start, Msg: "unterminated string literal"}
			}
			if c == quote {
				if triple {
					if s.peek(1) == quote && s.peek(2) == quote {
						s.advance(3)
						return Token{Kind: STRING, Value: prefix + string(s.src[startOff:s.pos]), Pos: start, End: s.Position()}, nil
					}
					s.advance(1)
					continue
				}
				s.advance(1)
				return Token{Kind: STRING, Value: prefix + string(s.src[startOff:s.pos]), Pos: start, End: s.Position()}, nil
			}
			s.advance(1)
			continue
		}
		// Format-spec mode: characters are literal text except for a
		// nested `{` (recursive replacement field), the matching `}`
		// that ends the interpolation, and a newline that terminates a
		// non-triple-quoted f-string.
		if inSpec {
			switch c {
			case '{':
				if s.peek(1) == '{' {
					s.advance(2)
					continue
				}
				depth++
				stack = append(stack, frame{brace: true})
				inSpec = false
				s.advance(1)
			case '}':
				// `}}` escape in spec mode is only honored when this is
				// the outermost spec frame. With a parent frame also in
				// spec mode (nested replacement field like
				// `f'{a:{b:{c:0}}}'`), the next `}` must close the inner
				// field rather than be paired with this one as a literal.
				outermostSpec := true
				for i := 0; i < len(stack)-1; i++ {
					if stack[i].brace && stack[i].spec {
						outermostSpec = false
						break
					}
				}
				if outermostSpec && s.peek(1) == '}' {
					s.advance(2)
					continue
				}
				depth--
				if n := len(stack); n > 0 {
					stack = stack[:n-1]
				}
				if n := len(stack); n > 0 {
					inSpec = stack[n-1].brace && stack[n-1].spec
				} else {
					inSpec = false
				}
				s.advance(1)
			case '\n':
				if !triple {
					return Token{}, &Error{Pos: start, Msg: "unterminated string literal"}
				}
				s.advance(1)
			default:
				s.advance(1)
			}
			continue
		}
		// Inside an interpolation expression. Track nesting; recurse into
		// nested string literals so a `"` inside doesn't escape us.
		switch c {
		case '{', '(', '[':
			// A `{` seen in expression mode is a dict/set literal opener,
			// not a replacement-field opener. Mark it brace=false so a
			// `:` inside (a dict key/value separator) doesn't get
			// mistaken for a format-spec colon.
			depth++
			stack = append(stack, frame{brace: false})
			s.advance(1)
		case '}', ')', ']':
			depth--
			if n := len(stack); n > 0 {
				stack = stack[:n-1]
			}
			if n := len(stack); n > 0 {
				inSpec = stack[n-1].brace && stack[n-1].spec
			} else {
				inSpec = false
			}
			s.advance(1)
		case '\'', '"':
			// Determine whether the nested string is itself an f/t
			// string by scanning back over the (possibly empty) string
			// prefix that immediately precedes this quote. A nested
			// plain string (`f'{"{"}'`) must not treat `{` as an
			// interpolation; a nested f-string (`f'{f"x={x}"}'`) must.
			pref := nestedPrefix(s.src, s.pos)
			if err := s.skipNestedString(start, pref); err != nil {
				return Token{}, err
			}
		case ':':
			s.advance(1)
			// Enter spec mode only when the innermost opener is a `{`
			// (a colon inside `[...]` or `(...)` is a slice or call
			// keyword, not a format spec).
			if n := len(stack); n > 0 && stack[n-1].brace && !stack[n-1].spec {
				stack[n-1].spec = true
				inSpec = true
			}
		case '#':
			// Comment inside an interpolation expression: only legal in
			// triple-quoted f-strings (PEP 701). Skip to end of line.
			for !s.Done() && s.peek(0) != '\n' {
				s.advance(1)
			}
		default:
			s.advance(1)
		}
	}
}

// nestedPrefix scans backwards from the byte before pos collecting an
// alphabetic string prefix (the letters that may legally precede a
// quote, like `r`, `f`, `rb`, `fr`). Returns "" if no prefix.
func nestedPrefix(src []byte, pos int) string {
	end := pos
	i := pos
	for i > 0 {
		c := src[i-1]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			i--
			continue
		}
		break
	}
	if end-i > 4 {
		return ""
	}
	return string(src[i:end])
}

// skipNestedString advances past a nested string literal that appears
// inside an interpolation. The nested literal may itself be an f/t
// string with its own interpolations; the prefix lets us decide whether
// to treat `{ ... }` inside the nested string as an interpolation.
func (s *Scanner) skipNestedString(outer Position, prefix string) error {
	interp := hasInterpolatedPrefix(prefix)
	// Detect optional string prefix: a short run of letters followed by a
	// quote. The interpolation's preceding tokens may have left a NAME-
	// looking prefix immediately before this quote, but that isn't our
	// problem — we only need to handle the quote at s.pos.
	q := s.peek(0)
	triple := false
	if s.peek(1) == q && s.peek(2) == q {
		triple = true
		s.advance(3)
	} else {
		s.advance(1)
	}
	for {
		if s.Done() {
			return &Error{Pos: outer, Msg: "unterminated string literal inside interpolation"}
		}
		c := s.peek(0)
		if c == '\\' {
			s.advance(2)
			continue
		}
		if interp {
			if c == '{' && s.peek(1) != '{' {
				// Recursive interpolation. Skip its expression.
				s.advance(1)
				if err := s.skipNestedInterpolation(outer); err != nil {
					return err
				}
				continue
			}
			if c == '{' && s.peek(1) == '{' {
				s.advance(2)
				continue
			}
			if c == '}' && s.peek(1) == '}' {
				s.advance(2)
				continue
			}
		}
		if c == q {
			if triple {
				if s.peek(1) == q && s.peek(2) == q {
					s.advance(3)
					return nil
				}
				s.advance(1)
				continue
			}
			s.advance(1)
			return nil
		}
		if c == '\n' && !triple {
			return &Error{Pos: outer, Msg: "unterminated string literal inside interpolation"}
		}
		s.advance(1)
	}
}

// skipNestedInterpolation skips one `{ ... }` group inside a nested
// f-string. Symmetric to the inner branch of scanInterpolatedString.
func (s *Scanner) skipNestedInterpolation(outer Position) error {
	depth := 1
	for depth > 0 {
		if s.Done() {
			return &Error{Pos: outer, Msg: "unterminated interpolation"}
		}
		c := s.peek(0)
		switch c {
		case '\\':
			s.advance(2)
		case '{', '(', '[':
			depth++
			s.advance(1)
		case '}', ')', ']':
			depth--
			s.advance(1)
		case '\'', '"':
			pref := nestedPrefix(s.src, s.pos)
			if err := s.skipNestedString(outer, pref); err != nil {
				return err
			}
		default:
			s.advance(1)
		}
	}
	return nil
}

func hasInterpolatedPrefix(prefix string) bool {
	for i := 0; i < len(prefix); i++ {
		c := prefix[i] | 0x20
		if c == 'f' || c == 't' {
			return true
		}
	}
	return false
}

// scanFlatString reads a string literal with no expression interpolation.
// It supports triple-quoted strings and the standard escape sequences.
func (s *Scanner) scanFlatString(start Position, prefix string) (Token, error) {
	startOff := s.pos
	quote := s.peek(0)
	triple := false
	if s.peek(1) == quote && s.peek(2) == quote {
		triple = true
		s.advance(3)
	} else {
		s.advance(1)
	}
	raw := isRawPrefix(prefix)
	for {
		if s.Done() {
			return Token{}, &Error{Pos: start, Msg: "unterminated string literal"}
		}
		c := s.peek(0)
		if c == '\\' {
			s.advance(1)
			if !s.Done() {
				if !raw {
					s.advance(1)
				} else {
					// In raw strings the backslash is preserved verbatim, but
					// `\` still escapes the following char from terminating
					// the string (so `r"\""` is a 2-char string).
					s.advance(1)
				}
			}
			continue
		}
		if c == '\n' && !triple {
			return Token{}, &Error{Pos: start, Msg: "unterminated string literal"}
		}
		if c == quote {
			if triple {
				if s.peek(1) == quote && s.peek(2) == quote {
					s.advance(3)
					return Token{
						Kind:  STRING,
						Value: prefix + string(s.src[startOff:s.pos]),
						Pos:   start,
						End:   s.Position(),
					}, nil
				}
				s.advance(1)
				continue
			}
			s.advance(1)
			return Token{
				Kind:  STRING,
				Value: prefix + string(s.src[startOff:s.pos]),
				Pos:   start,
				End:   s.Position(),
			}, nil
		}
		s.advance(1)
	}
}

func isRawPrefix(prefix string) bool {
	for i := 0; i < len(prefix); i++ {
		if prefix[i] == 'r' || prefix[i] == 'R' {
			return true
		}
	}
	return false
}
