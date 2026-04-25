package lex

// Indent processes a stream of physical tokens (from Scanner) and produces
// the logical token stream Python's grammar references: with NEWLINE
// emitted at the end of each logical line, INDENT/DEDENT around indented
// blocks, comments dropped (including TYPE_COMMENT, until a consumer is
// added for them), and a single ENDMARKER at EOF.
//
// Logical lines collapse over open brackets — `(`, `[`, `{` suppress NEWLINE
// emission until the matching closer. Backslash-NL is already handled by the
// scanner. Blank lines are ignored.
type Indent struct {
	scan    *Scanner
	indents []int // indent stack; always starts with 0
	// pending is a FIFO queue of tokens to emit before the next physical
	// scan. pendingHead is the index of the next token to emit; the slice
	// is reset to length 0 (preserving the backing array) when the head
	// catches up with the tail. This avoids the alloc-storm that
	// `pending = pending[1:]` produces on a 1800-file corpus, where the
	// header slide leaks the prefix and the next append reallocates.
	pending     []Token
	pendingHead int
	bracket     int  // open bracket depth
	lineStart   bool // true when the next token is the first non-WS on a line
	colHint     int  // column of the first significant token on the current line
	emittedEnd  bool
	// keepComments toggles whether COMMENT and TYPE_COMMENT tokens are
	// passed through. The parser path leaves it false (the grammar has
	// no rule that consumes COMMENT). The cst package sets it true so
	// it can show comments to downstream tools.
	keepComments bool
}

// NewIndent wraps a Scanner and returns an Indent that yields the
// logical token stream Python's grammar consumes (NEWLINE, INDENT,
// DEDENT, ENDMARKER injected; comments dropped).
func NewIndent(s *Scanner) *Indent {
	return &Indent{scan: s, indents: []int{0}, lineStart: true}
}

// Next returns the next logical token, or EOF after ENDMARKER has been
// emitted.
func (it *Indent) Next() (Token, error) {
	if it.pendingHead < len(it.pending) {
		return it.dequeue(), nil
	}
	if it.emittedEnd {
		return Token{Kind: EOF}, nil
	}
	for {
		tok, err := it.scan.Scan()
		if err != nil {
			return Token{}, err
		}

		// Drop comments. TYPE_COMMENT is recognised by the scanner but
		// no grammar rule consumes it yet, so we drop those too rather
		// than letting them break the parse. The cst package flips
		// keepComments so it can preserve them.
		if tok.Kind == COMMENT || tok.Kind == TYPE_COMMENT {
			if it.keepComments {
				return tok, nil
			}
			continue
		}

		if tok.Kind == EOF {
			return it.flushAtEnd()
		}

		// Track open/close brackets to suppress NEWLINE inside them. The
		// bracket count is incremented *after* the lineStart check below,
		// so a `(` that opens a logical line still triggers indent
		// processing against its own column.
		preBracket := it.bracket
		switch tok.Kind {
		case LPAREN, LBRACK, LBRACE:
			it.bracket++
		case RPAREN, RBRACK, RBRACE:
			if it.bracket > 0 {
				it.bracket--
			}
		}

		if tok.Kind == NEWLINE {
			if preBracket > 0 {
				// inside brackets: ignore NEWLINE entirely
				continue
			}
			if it.lineStart {
				// blank line at top of new logical line: ignore
				continue
			}
			it.lineStart = true
			return tok, nil
		}

		// First non-NEWLINE token on a line: compare its column against the
		// indent stack and emit INDENT or DEDENT(s) accordingly. Inside open
		// brackets the indent check is suppressed, but lineStart still has
		// to be cleared so a later closer doesn't trigger a retroactive
		// indent dance against some inner token's column.
		if it.lineStart {
			it.lineStart = false
			if preBracket > 0 {
				return tok, nil
			}
			col := tok.Pos.Col
			top := it.indents[len(it.indents)-1]
			if col > top {
				it.indents = append(it.indents, col)
				it.queue(Token{Kind: INDENT, Pos: tok.Pos, End: tok.Pos})
			} else if col < top {
				for col < it.indents[len(it.indents)-1] {
					it.indents = it.indents[:len(it.indents)-1]
					it.queue(Token{Kind: DEDENT, Pos: tok.Pos, End: tok.Pos})
				}
				if col != it.indents[len(it.indents)-1] {
					return Token{}, &Error{Pos: tok.Pos, Msg: "unindent does not match any outer indentation level"}
				}
			}
			// queue the token after any indent dance
			it.queue(tok)
			return it.dequeue(), nil
		}

		return tok, nil
	}
}

// queue appends a token to the pending queue.
func (it *Indent) queue(t Token) { it.pending = append(it.pending, t) }

// dequeue removes and returns the head of the pending queue. When the
// head catches up with the tail the slice is reset (preserving the
// backing array) so subsequent queues reuse capacity instead of
// allocating.
func (it *Indent) dequeue() Token {
	t := it.pending[it.pendingHead]
	it.pendingHead++
	if it.pendingHead == len(it.pending) {
		it.pending = it.pending[:0]
		it.pendingHead = 0
	}
	return t
}

// flushAtEnd emits a final NEWLINE (if needed), all outstanding DEDENTs, then
// ENDMARKER.
func (it *Indent) flushAtEnd() (Token, error) {
	pos := it.scan.Position()
	if !it.lineStart {
		it.queue(Token{Kind: NEWLINE, Value: "\n", Pos: pos, End: pos})
		it.lineStart = true
	}
	for len(it.indents) > 1 {
		it.indents = it.indents[:len(it.indents)-1]
		it.queue(Token{Kind: DEDENT, Pos: pos, End: pos})
	}
	it.queue(Token{Kind: ENDMARKER, Pos: pos, End: pos})
	it.emittedEnd = true
	return it.dequeue(), nil
}

// All drains the iterator into a slice. Useful for tests and tooling.
func (it *Indent) All() ([]Token, error) {
	var out []Token
	for {
		t, err := it.Next()
		if err != nil {
			return nil, err
		}
		out = append(out, t)
		if t.Kind == ENDMARKER {
			return out, nil
		}
	}
}
