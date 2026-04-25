package parser

import (
	"bytes"
	"fmt"

	"github.com/alecthomas/participle/v2"
)

// builtParser is the lazily-built participle parser. It's cached because
// participle's Build() does table construction work we don't want to repeat
// per-call.
var builtParser *participle.Parser[File]

// build constructs the participle parser, panicking on grammar error. We
// panic rather than return an error because the grammar is static — if it
// fails, no input can succeed and the package itself is broken.
func build() *participle.Parser[File] {
	if builtParser != nil {
		return builtParser
	}
	p, err := participle.Build[File](
		participle.Lexer(NewLexerDefinition()),
		participle.UseLookahead(8),
		// participle's default is to elide whitespace; we don't want that —
		// our lexer already strips whitespace, and we need to see every
		// token (especially NEWLINE/INDENT/DEDENT) verbatim.
	)
	if err != nil {
		panic(fmt.Sprintf("gopapy parser: grammar build failed: %v", err))
	}
	builtParser = p
	return p
}

// ParseFile parses src as a Python module body and returns the
// participle parse tree. filename is used only for error messages.
// On a syntax error the returned error is a participle.Error with
// position information.
func ParseFile(filename string, src []byte) (*File, error) {
	p := build()
	return p.ParseBytes(filename, src)
}

// ParseString is a convenience wrapper around ParseFile that takes
// src as a string instead of a byte slice.
func ParseString(filename, src string) (*File, error) {
	return ParseFile(filename, []byte(src))
}

// ParseExpression parses src as a single Python expression and returns
// the participle Expression node. The source is wrapped as a one-line
// statement and run through the file grammar, then unwrapped. An error
// is returned if src does not parse as a bare expression.
func ParseExpression(src string) (*Expression, error) {
	f, err := ParseFile("<fstring>", []byte(src+"\n"))
	if err != nil {
		return nil, err
	}
	if len(f.Statements) == 0 || f.Statements[0].Simples == nil {
		return nil, fmt.Errorf("not an expression: %q", src)
	}
	s := f.Statements[0].Simples.First
	switch {
	case s.ExprStmt != nil:
		return s.ExprStmt, nil
	case s.Assign != nil && s.Assign.Annot == nil && s.Assign.Aug == "" && len(s.Assign.More) == 0:
		// Bare expression that parsed through the assignment alternative
		// because Assign is tried first. Unwrap the Target.
		t := s.Assign.Target
		if t != nil && len(t.Tail) == 0 && !t.HasTrail && !t.Head.Star {
			return t.Head.Expr, nil
		}
	}
	return nil, fmt.Errorf("not an expression: %q", src)
}

// ParseReader parses src as a Python module body. It clones src before
// parsing so callers may reuse or mutate the buffer afterward. Provided
// for API symmetry with the standard library; if you already own the
// bytes, ParseFile is cheaper.
func ParseReader(filename string, src []byte) (*File, error) {
	return ParseFile(filename, bytes.Clone(src))
}
