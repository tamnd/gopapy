// Package lex tokenises Python 3.14 source. It produces a stream that the
// parser package consumes via participle's lexer.Definition adapter.
//
// The output mirrors CPython's tokenize module: physical tokens for
// names, numbers, strings, and operators, plus the synthetic NEWLINE,
// INDENT, DEDENT, and ENDMARKER tokens that Python's grammar references
// directly.
//
// NewScanner and NewIndent are stable entry points from v0.1.0 onward
// within the v1 module path.
package lex

import "fmt"

// Kind is a token kind. Values are stable; the parser uses them as
// participle TokenType identifiers.
type Kind int

const (
	// Sentinel + structural tokens.
	EOF Kind = iota
	NEWLINE
	INDENT
	DEDENT
	ENDMARKER
	COMMENT // attached for tooling, ignored by the parser

	// Literals and names.
	NAME
	NUMBER
	STRING
	FSTRING_START
	FSTRING_MIDDLE
	FSTRING_END
	TSTRING_START
	TSTRING_MIDDLE
	TSTRING_END
	TYPE_COMMENT // `# type: ...` comments

	// Operators and delimiters. Python has 34 of them.
	PLUS         // +
	MINUS        // -
	STAR         // *
	DOUBLESTAR   // **
	SLASH        // /
	DOUBLESLASH  // //
	PERCENT      // %
	AT           // @
	AMP          // &
	PIPE         // |
	CARET        // ^
	TILDE        // ~
	LSHIFT       // <<
	RSHIFT       // >>
	LT           // <
	GT           // >
	LE           // <=
	GE           // >=
	EQEQ         // ==
	NE           // !=
	EQ           // =
	WALRUS       // :=
	PLUSEQ       // +=
	MINUSEQ      // -=
	STAREQ       // *=
	SLASHEQ      // /=
	DOUBLESLEQ   // //=
	PERCENTEQ    // %=
	ATEQ         // @=
	AMPEQ        // &=
	PIPEEQ       // |=
	CARETEQ      // ^=
	LSHIFTEQ     // <<=
	RSHIFTEQ     // >>=
	DOUBLESTAREQ // **=
	LPAREN       // (
	RPAREN       // )
	LBRACK       // [
	RBRACK       // ]
	LBRACE       // {
	RBRACE       // }
	COMMA        // ,
	COLON        // :
	SEMI         // ;
	DOT          // .
	ELLIPSIS     // ...
	ARROW        // ->
)

// Position is a 1-indexed source position.
// Col is a 0-indexed UTF-8 byte offset on the line, matching CPython's
// `col_offset` exactly.
type Position struct {
	Filename string
	Offset   int // 0-indexed byte offset from start of file
	Line     int // 1-indexed
	Col      int // 0-indexed UTF-8 byte offset in line
}

// Token is one lexed token with its source span.
type Token struct {
	Kind  Kind
	Value string   // raw lexeme as it appeared in source (e.g. `"hello"`, `0xFF`)
	Pos   Position // start position
	End   Position // end position (exclusive)
}

func (t Token) String() string {
	return fmt.Sprintf("%s(%q) @ %d:%d", t.Kind, t.Value, t.Pos.Line, t.Pos.Col)
}

// String is for debug/diagnostic output. The names match CPython's tokenize
// module so cross-checking against `python -m tokenize` is straightforward.
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

var kindNames = map[Kind]string{
	EOF: "EOF", NEWLINE: "NEWLINE", INDENT: "INDENT", DEDENT: "DEDENT",
	ENDMARKER: "ENDMARKER", COMMENT: "COMMENT",
	NAME: "NAME", NUMBER: "NUMBER", STRING: "STRING",
	FSTRING_START: "FSTRING_START", FSTRING_MIDDLE: "FSTRING_MIDDLE", FSTRING_END: "FSTRING_END",
	TSTRING_START: "TSTRING_START", TSTRING_MIDDLE: "TSTRING_MIDDLE", TSTRING_END: "TSTRING_END",
	TYPE_COMMENT: "TYPE_COMMENT",
	PLUS:         "+", MINUS: "-", STAR: "*", DOUBLESTAR: "**",
	SLASH: "/", DOUBLESLASH: "//", PERCENT: "%", AT: "@",
	AMP: "&", PIPE: "|", CARET: "^", TILDE: "~",
	LSHIFT: "<<", RSHIFT: ">>",
	LT: "<", GT: ">", LE: "<=", GE: ">=", EQEQ: "==", NE: "!=",
	EQ: "=", WALRUS: ":=",
	PLUSEQ: "+=", MINUSEQ: "-=", STAREQ: "*=", SLASHEQ: "/=",
	DOUBLESLEQ: "//=", PERCENTEQ: "%=", ATEQ: "@=", AMPEQ: "&=",
	PIPEEQ: "|=", CARETEQ: "^=", LSHIFTEQ: "<<=", RSHIFTEQ: ">>=",
	DOUBLESTAREQ: "**=",
	LPAREN:       "(", RPAREN: ")", LBRACK: "[", RBRACK: "]",
	LBRACE: "{", RBRACE: "}",
	COMMA: ",", COLON: ":", SEMI: ";", DOT: ".", ELLIPSIS: "...", ARROW: "->",
}

// Error is a lex error with a source position attached.
type Error struct {
	Pos Position
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.Pos.Filename, e.Pos.Line, e.Pos.Col, e.Msg)
}
