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
	for {
		if s.Done() {
			return Token{}, &Error{Pos: start, Msg: "unterminated string literal"}
		}
		c := s.peek(0)
		if c == '\\' && !raw {
			s.advance(1)
			if !s.Done() {
				s.advance(1)
			}
			continue
		}
		if c == '\\' && raw {
			s.advance(2)
			continue
		}
		if depth == 0 {
			if c == '{' {
				if s.peek(1) == '{' {
					s.advance(2)
					continue
				}
				depth = 1
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
		// Inside an interpolation expression. Track nesting; recurse into
		// nested string literals so a `"` inside doesn't escape us.
		switch c {
		case '{', '(', '[':
			depth++
			s.advance(1)
		case '}', ')', ']':
			depth--
			s.advance(1)
		case '\'', '"':
			if err := s.skipNestedString(start); err != nil {
				return Token{}, err
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

// skipNestedString advances past a nested string literal that appears
// inside an interpolation. The nested literal may itself be an f/t
// string with its own interpolations, so we recurse via a temporary
// prefix scan.
func (s *Scanner) skipNestedString(outer Position) error {
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
			if err := s.skipNestedString(outer); err != nil {
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
