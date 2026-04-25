package lex

// Number literals.
//
// Python supports decimal integers, hex (0x), oct (0o), bin (0b), floats with
// optional exponent and fractional parts, complex/imaginary literals (suffix
// `j` or `J`), and PEP 515 underscores between digits. We keep the raw lexeme
// in the Token.Value — conversion to int/float/complex happens during AST
// emission, where we know whether a `j` makes the value imaginary.

func (s *Scanner) scanNumber(start Position) (Token, error) {
	startOff := s.pos
	// Integer or fractional part.
	if !s.scanDigitRun(false) {
		// Leading dot already implies a digit follows — caller checked.
	}
	hadDot := false
	if s.peek(0) == '.' {
		// `5.`, `5.0`, and `.5` all land here. We already consumed the
		// leading digit run (or none, in the `.5` case where caller checks
		// digit-after-dot). Accept a fractional run if any digits follow,
		// but a bare `5.` is also a valid float literal.
		if startOff == s.pos {
			// `.5` form — caller guarantees digit after dot.
			s.advance(1)
			s.scanDigitRun(false)
			hadDot = true
		} else {
			s.advance(1)
			s.scanDigitRun(false)
			hadDot = true
		}
	}
	// Exponent.
	if c := s.peek(0); c == 'e' || c == 'E' {
		s.advance(1)
		if c2 := s.peek(0); c2 == '+' || c2 == '-' {
			s.advance(1)
		}
		if !s.scanDigitRun(false) {
			return Token{}, &Error{Pos: s.Position(), Msg: "missing digits in exponent"}
		}
		hadDot = true // exponent makes it a float in Python
	}
	// Imaginary suffix.
	if c := s.peek(0); c == 'j' || c == 'J' {
		s.advance(1)
		_ = hadDot
	}
	return Token{Kind: NUMBER, Value: string(s.src[startOff:s.pos]), Pos: start, End: s.Position()}, nil
}

// scanRadix handles 0x/0o/0b prefixes. Underscores between digits are allowed.
func (s *Scanner) scanRadix(start Position) (Token, error) {
	startOff := s.pos
	s.advance(2) // 0x / 0o / 0b
	prev := s.src[s.pos-1]
	allow := func(b byte) bool {
		switch prev | 0x20 { // lowercase
		case 'x':
			return isHex(b)
		case 'o':
			return b >= '0' && b <= '7'
		case 'b':
			return b == '0' || b == '1'
		}
		return false
	}
	any := false
	for !s.Done() {
		c := s.peek(0)
		if c == '_' {
			// PEP 515: underscore allowed right after the base specifier
			// and between digits, as long as a valid digit follows.
			if !allow(s.peek(1)) {
				return Token{}, &Error{Pos: s.Position(), Msg: "invalid underscore in numeric literal"}
			}
			s.advance(1)
			continue
		}
		if !allow(c) {
			break
		}
		any = true
		s.advance(1)
	}
	if !any {
		return Token{}, &Error{Pos: s.Position(), Msg: "missing digits after radix prefix"}
	}
	return Token{Kind: NUMBER, Value: string(s.src[startOff:s.pos]), Pos: start, End: s.Position()}, nil
}

// scanDigitRun consumes /[0-9](_?[0-9])*/. Returns true iff at least one
// digit was consumed. allowLeadingZero is currently unused — Python permits
// leading zeros only in 0/0_0/etc.; we accept them and let the AST pass
// reject `0123`-style decimals (CPython's behaviour).
func (s *Scanner) scanDigitRun(allowLeadingZero bool) bool {
	_ = allowLeadingZero
	any := false
	for !s.Done() {
		c := s.peek(0)
		if c == '_' && isDigit(s.peek(1)) {
			s.advance(1)
			continue
		}
		if !isDigit(c) {
			break
		}
		any = true
		s.advance(1)
	}
	return any
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
