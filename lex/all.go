package lex

// AllTokens returns every logical token from src — including COMMENT and
// TYPE_COMMENT tokens that the regular Indent layer drops. INDENT,
// DEDENT, NEWLINE, and ENDMARKER are injected the same way Indent does.
//
// This is the entry point the cst package uses to keep comments
// available to downstream tools. The parser path keeps using NewIndent
// directly because the grammar has no rule that consumes COMMENT.
func AllTokens(filename string, src []byte) ([]Token, error) {
	it := newIndentKeepComments(NewScanner(src, filename))
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

// newIndentKeepComments wraps a Scanner with the indent layer in
// "preserve comments" mode. The implementation reuses Indent's logic
// by toggling a private field rather than duplicating the state
// machine.
func newIndentKeepComments(s *Scanner) *Indent {
	i := NewIndent(s)
	i.keepComments = true
	return i
}
