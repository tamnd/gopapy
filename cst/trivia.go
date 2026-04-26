package cst

import (
	"github.com/tamnd/gopapy/v1/ast"
	"github.com/tamnd/gopapy/v1/lex"
)

// Position labels how a Comment relates to its host AST node.
type Position int

const (
	// Leading is for comments on their own line(s) before the node.
	Leading Position = iota
	// Trailing is for comments on the same line as the node.
	Trailing
)

// Comment is one COMMENT or TYPE_COMMENT token captured with the
// position label that says how it attaches to its host node.
type Comment struct {
	Pos      lex.Position
	Text     string
	Position Position
}

// Trivia is the side table produced by File.AttachComments. It maps
// each AST node that owns at least one comment to those comments in
// source order, plus a File slice for comments that attach to no
// statement (typically end-of-file comments after the last stmt).
type Trivia struct {
	ByNode map[ast.Node][]Comment
	File   []Comment
}

// AttachComments walks the token stream once and links every COMMENT
// and TYPE_COMMENT to the AST node it logically belongs to. The rules
// match the v0.1.9 spec (notes/Spec/1100/1137):
//
//   - Trailing: a comment that follows a non-comment token on the same
//     line is attached to the innermost statement that ends on that
//     line.
//   - Leading: a comment alone on its line attaches to the next
//     statement that begins on a later line.
//   - End of file: comments that match neither rule (no following
//     statement) land in Trivia.File.
//
// AttachComments returns a fresh Trivia per call. The CST source bytes
// and the AST itself are not mutated.
func (f *File) AttachComments() *Trivia {
	stmts := collectStmts(f.AST)
	trivia := &Trivia{ByNode: map[ast.Node][]Comment{}}

	// pendingLeading buffers a run of consecutive leading-comment
	// lines. They all attach to the next statement we find.
	var pendingLeading []Comment

	// prevNonCommentLine tracks the line of the most recent non-comment,
	// non-whitespace token. Used to decide trailing vs leading: if the
	// current comment shares that line, it's trailing.
	prevNonCommentLine := -1

	for _, tk := range f.tokens {
		switch tk.Kind {
		case lex.COMMENT, lex.TYPE_COMMENT:
			c := Comment{Pos: tk.Pos, Text: tk.Value}
			if tk.Pos.Line == prevNonCommentLine {
				c.Position = Trailing
				if owner := innermostStmtEndingOnLine(stmts, tk.Pos.Line); owner != nil {
					trivia.ByNode[owner] = append(trivia.ByNode[owner], c)
				} else {
					trivia.File = append(trivia.File, c)
				}
			} else {
				c.Position = Leading
				pendingLeading = append(pendingLeading, c)
			}
		case lex.NEWLINE, lex.INDENT, lex.DEDENT, lex.ENDMARKER:
			// Layout-only tokens don't end a logical "non-comment" line
			// for trailing-comment purposes.
		default:
			// A real token on a real line. Drain pending leading
			// comments onto the statement that owns this token (the
			// outermost statement starting on this line — same line as
			// the leading run's first following statement).
			if len(pendingLeading) > 0 {
				if owner := outermostStmtStartingOnOrAfterLine(stmts, tk.Pos.Line); owner != nil {
					trivia.ByNode[owner] = append(trivia.ByNode[owner], pendingLeading...)
				} else {
					trivia.File = append(trivia.File, pendingLeading...)
				}
				pendingLeading = nil
			}
			prevNonCommentLine = tk.Pos.Line
		}
	}

	// Anything still pending after ENDMARKER had no following stmt.
	if len(pendingLeading) > 0 {
		trivia.File = append(trivia.File, pendingLeading...)
	}

	return trivia
}

// collectStmts gathers every StmtNode in the module in source order.
// Pre-order is fine — outer statements come before their inner body
// statements, which is what the lookup helpers below want.
func collectStmts(mod *ast.Module) []ast.StmtNode {
	if mod == nil {
		return nil
	}
	var out []ast.StmtNode
	ast.WalkPreorder(mod, func(n ast.Node) {
		if s, ok := n.(ast.StmtNode); ok {
			out = append(out, s)
		}
	})
	return out
}

// innermostStmtEndingOnLine returns the deepest (last in pre-order)
// statement whose end line equals line. Pre-order puts parents before
// children, so iterating forward and keeping the last match yields
// the innermost.
func innermostStmtEndingOnLine(stmts []ast.StmtNode, line int) ast.StmtNode {
	var match ast.StmtNode
	for _, s := range stmts {
		p := s.GetPos()
		if p.EndLineno == line {
			match = s
		}
	}
	return match
}

// outermostStmtStartingOnOrAfterLine returns the first statement in
// source order whose start line is >= line. Pre-order means the
// outermost statement is hit first.
func outermostStmtStartingOnOrAfterLine(stmts []ast.StmtNode, line int) ast.StmtNode {
	for _, s := range stmts {
		if s.GetPos().Lineno >= line {
			return s
		}
	}
	return nil
}
