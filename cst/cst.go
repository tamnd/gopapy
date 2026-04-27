// Package cst is a thin layer above the AST that preserves the
// original source bytes and the full token stream (including
// comments). It is the foundation for tools — formatters, codemods,
// linters that need source location — that cannot work from the
// canonicalised AST alone.
//
// The contract in this version is intentionally narrow: a CST File
// carries the original source unchanged, the AST module produced from
// it, and the complete token sequence with comments included. Trivia
// attached to specific AST nodes, end-position computation, and
// mutation surfaces are deferred to later versions; see the v0.1.3
// spec at notes/Spec/1100/1130 for the rationale.
package cst

import (
	"bytes"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/lex"
	"github.com/tamnd/gopapy/legacy/parser"
)

// File is a parsed Python source file together with the original
// source bytes and the unfiltered token stream.
type File struct {
	Filename string
	source   []byte
	AST      *ast.Module
	tokens   []lex.Token
}

// Parse reads src as Python source and returns a CST File. The
// source bytes are cloned so the caller may mutate the original
// buffer without affecting the File.
func Parse(filename string, src []byte) (*File, error) {
	clone := bytes.Clone(src)
	pf, err := parser.ParseFile(filename, clone)
	if err != nil {
		return nil, err
	}
	tokens, err := lex.AllTokens(filename, clone)
	if err != nil {
		return nil, err
	}
	return &File{
		Filename: filename,
		source:   clone,
		AST:      ast.FromFile(pf),
		tokens:   tokens,
	}, nil
}

// Source returns the original source bytes the File was parsed from.
// The slice is owned by the File; callers must not mutate it.
func (f *File) Source() []byte { return f.source }

// Tokens returns the full token stream from the source, including
// COMMENT and TYPE_COMMENT tokens that the parser drops. The slice
// is owned by the File; callers must not mutate it.
func (f *File) Tokens() []lex.Token { return f.tokens }
