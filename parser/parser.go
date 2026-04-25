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

// ParseFile parses src as a Python module body. filename is used only for
// error messages.
func ParseFile(filename string, src []byte) (*File, error) {
	p := build()
	return p.ParseBytes(filename, src)
}

// ParseString is a convenience wrapper around ParseFile.
func ParseString(filename, src string) (*File, error) {
	return ParseFile(filename, []byte(src))
}

// ParseReader parses from a byte slice presented as a buffer (kept here for
// API symmetry with the std library — in practice all callers have bytes
// already).
func ParseReader(filename string, src []byte) (*File, error) {
	return ParseFile(filename, bytes.Clone(src))
}
