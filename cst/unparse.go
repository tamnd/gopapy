package cst

import (
	"strings"

	"github.com/tamnd/gopapy/v1/ast"
)

// Unparse renders the file back to Python source, weaving the
// comments captured by AttachComments back into the output stream.
// Files without comments produce the same byte sequence as
// ast.Unparse(f.AST). Files with comments preserve each leading
// comment on its own line above the host statement and each trailing
// comment on the same line as the statement, separated by two
// spaces. Comments not attached to any statement (typically end-of-
// file blocks) land at module scope after the last statement.
func (f *File) Unparse() string {
	hooks := &triviaHooks{trivia: f.AttachComments()}
	return ast.UnparseWith(f.AST, hooks)
}

type triviaHooks struct {
	trivia *Trivia
}

func (h *triviaHooks) LeadingFor(s ast.StmtNode) []string {
	cmts := h.trivia.ByNode[s]
	if len(cmts) == 0 {
		return nil
	}
	var out []string
	for _, c := range cmts {
		if c.Position == Leading {
			out = append(out, c.Text)
		}
	}
	return out
}

func (h *triviaHooks) TrailingFor(s ast.StmtNode) string {
	cmts := h.trivia.ByNode[s]
	if len(cmts) == 0 {
		return ""
	}
	var trailing []string
	for _, c := range cmts {
		if c.Position == Trailing {
			trailing = append(trailing, c.Text)
		}
	}
	if len(trailing) == 0 {
		return ""
	}
	return strings.Join(trailing, "  ")
}

func (h *triviaHooks) FileTrailing() []string {
	if len(h.trivia.File) == 0 {
		return nil
	}
	out := make([]string, 0, len(h.trivia.File))
	for _, c := range h.trivia.File {
		out = append(out, c.Text)
	}
	return out
}
