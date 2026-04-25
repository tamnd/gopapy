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
		// f-string / t-string: lexed as a flat STRING token here so the
		// bootstrap parser can treat them like ordinary strings until the
		// stateful lexer arrives. The AST emitter rejects them with a clear
		// "f-strings not yet supported in PR1" diagnostic.
		return s.scanFlatString(start, prefix)
	}
	return s.scanFlatString(start, prefix)
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
