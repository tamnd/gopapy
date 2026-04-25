// Package parser turns a Python token stream into a tree shaped by
// participle, then emits an AST compatible with CPython's ast module.
package parser

import (
	"fmt"
	"io"

	plexer "github.com/alecthomas/participle/v2/lexer"

	"github.com/tamnd/gopapy/lex"
)

// definition adapts our lex package to participle's lexer.Definition.
type definition struct{}

// NewLexerDefinition returns a participle lexer.Definition that reads Python
// source via lex.Scanner + lex.Indent.
func NewLexerDefinition() plexer.Definition { return &definition{} }

// Symbols maps participle's symbolic token names to our lex.Kind values
// (which we cast to participle's TokenType). The grammar refers to tokens
// by these symbol names, e.g. `@@NAME` captures a NAME token.
func (definition) Symbols() map[string]plexer.TokenType {
	out := map[string]plexer.TokenType{}
	for k, name := range tokenSymbols {
		out[name] = plexer.TokenType(k)
	}
	out["EOF"] = plexer.EOF
	return out
}

// Lex reads from r and returns a participle Lexer over our logical token
// stream (NEWLINE/INDENT/DEDENT injected, comments dropped).
func (definition) Lex(filename string, r io.Reader) (plexer.Lexer, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	it := lex.NewIndent(lex.NewScanner(src, filename))
	return &lexerAdapter{it: it, filename: filename}, nil
}

type lexerAdapter struct {
	it       *lex.Indent
	filename string
	done     bool
}

// Next returns the next participle token, or EOF.
func (a *lexerAdapter) Next() (plexer.Token, error) {
	if a.done {
		return plexer.Token{Type: plexer.EOF}, nil
	}
	t, err := a.it.Next()
	if err != nil {
		return plexer.Token{}, err
	}
	if t.Kind == lex.EOF {
		a.done = true
		return plexer.Token{Type: plexer.EOF, Pos: a.adapt(t.Pos)}, nil
	}
	if t.Kind == lex.ENDMARKER {
		// participle treats EOF as the natural terminator. Pass through
		// ENDMARKER as itself; the grammar can `~ENDMARKER` if it wants
		// to consume it. The next Next() returns plexer.EOF.
		a.done = true
		tok := plexer.Token{
			Type:  plexer.TokenType(t.Kind),
			Value: t.Value,
			Pos:   a.adapt(t.Pos),
		}
		return tok, nil
	}
	val := t.Value
	if val == "" {
		val = lex.Kind(t.Kind).String()
	}
	return plexer.Token{
		Type:  plexer.TokenType(t.Kind),
		Value: val,
		Pos:   a.adapt(t.Pos),
	}, nil
}

func (a *lexerAdapter) adapt(p lex.Position) plexer.Position {
	return plexer.Position{
		Filename: a.filename,
		Offset:   p.Offset,
		Line:     p.Line,
		Column:   p.Col + 1, // participle uses 1-indexed; lex uses 0-indexed
	}
}

// tokenSymbols maps lex.Kind to a participle-friendly symbol name. The
// names show up in error messages, so they're kept human-readable. The set
// must agree exactly with what the grammar struct tags reference.
var tokenSymbols = map[lex.Kind]string{
	lex.NEWLINE:      "NEWLINE",
	lex.INDENT:       "INDENT",
	lex.DEDENT:       "DEDENT",
	lex.ENDMARKER:    "ENDMARKER",
	lex.NAME:         "NAME",
	lex.NUMBER:       "NUMBER",
	lex.STRING:       "STRING",
	lex.TYPE_COMMENT: "TYPE_COMMENT",

	lex.PLUS: "PLUS", lex.MINUS: "MINUS",
	lex.STAR: "STAR", lex.DOUBLESTAR: "DOUBLESTAR",
	lex.SLASH: "SLASH", lex.DOUBLESLASH: "DOUBLESLASH",
	lex.PERCENT: "PERCENT", lex.AT: "AT",
	lex.AMP: "AMP", lex.PIPE: "PIPE", lex.CARET: "CARET", lex.TILDE: "TILDE",
	lex.LSHIFT: "LSHIFT", lex.RSHIFT: "RSHIFT",
	lex.LT: "LT", lex.GT: "GT", lex.LE: "LE", lex.GE: "GE",
	lex.EQEQ: "EQEQ", lex.NE: "NE",
	lex.EQ: "EQ", lex.WALRUS: "WALRUS",
	lex.PLUSEQ: "PLUSEQ", lex.MINUSEQ: "MINUSEQ",
	lex.STAREQ: "STAREQ", lex.SLASHEQ: "SLASHEQ",
	lex.DOUBLESLEQ: "DOUBLESLEQ", lex.PERCENTEQ: "PERCENTEQ", lex.ATEQ: "ATEQ",
	lex.AMPEQ: "AMPEQ", lex.PIPEEQ: "PIPEEQ", lex.CARETEQ: "CARETEQ",
	lex.LSHIFTEQ: "LSHIFTEQ", lex.RSHIFTEQ: "RSHIFTEQ", lex.DOUBLESTAREQ: "DOUBLESTAREQ",
	lex.LPAREN: "LPAREN", lex.RPAREN: "RPAREN",
	lex.LBRACK: "LBRACK", lex.RBRACK: "RBRACK",
	lex.LBRACE: "LBRACE", lex.RBRACE: "RBRACE",
	lex.COMMA: "COMMA", lex.COLON: "COLON", lex.SEMI: "SEMI",
	lex.DOT: "DOT", lex.ELLIPSIS: "ELLIPSIS", lex.ARROW: "ARROW",
}

// SymbolByName looks up a symbol by name. Test utility.
func SymbolByName(name string) (plexer.TokenType, bool) {
	for k, n := range tokenSymbols {
		if n == name {
			return plexer.TokenType(k), true
		}
	}
	return 0, false
}

// Sanity check at init: the symbol table must be non-empty and contain no
// duplicates. The exact entry count is allowed to grow as new token kinds
// land (FSTRING_*, TSTRING_*, ...).
func init() {
	if len(tokenSymbols) == 0 {
		panic("tokenSymbols empty")
	}
	seen := map[string]bool{}
	for _, name := range tokenSymbols {
		if seen[name] {
			panic(fmt.Sprintf("duplicate token symbol %q", name))
		}
		seen[name] = true
	}
}
