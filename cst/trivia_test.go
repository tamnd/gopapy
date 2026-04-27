package cst

import (
	"reflect"
	"testing"

	"github.com/tamnd/gopapy/ast"
)

// parseTrivia is the small helper every trivia test reaches for: parse
// the source, attach comments, return both the file and the trivia so
// tests can poke at AST nodes directly.
func parseTrivia(t *testing.T, src string) (*File, *Trivia) {
	t.Helper()
	f, err := Parse("<test>", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return f, f.AttachComments()
}

// firstStmt returns the first top-level statement of the module so a
// test can assert "this comment attaches to that stmt" without
// hand-walking Body each time.
func firstStmt(t *testing.T, f *File) ast.StmtNode {
	t.Helper()
	if f.AST == nil || len(f.AST.Body) == 0 {
		t.Fatal("module has no body")
	}
	return f.AST.Body[0]
}

func TestAttach_TrailingInline(t *testing.T) {
	f, tr := parseTrivia(t, "x = 1  # tail\n")
	stmt := firstStmt(t, f)
	got := tr.ByNode[stmt]
	if len(got) != 1 {
		t.Fatalf("expected 1 comment on stmt, got %d (%v)", len(got), got)
	}
	if got[0].Position != Trailing {
		t.Errorf("position = %v, want Trailing", got[0].Position)
	}
	if got[0].Text != "# tail" {
		t.Errorf("text = %q, want %q", got[0].Text, "# tail")
	}
	if len(tr.File) != 0 {
		t.Errorf("File should be empty, got %v", tr.File)
	}
}

func TestAttach_LeadingSingleLine(t *testing.T) {
	f, tr := parseTrivia(t, "# doc\nx = 1\n")
	stmt := firstStmt(t, f)
	got := tr.ByNode[stmt]
	if len(got) != 1 {
		t.Fatalf("expected 1 comment on stmt, got %d (%v)", len(got), got)
	}
	if got[0].Position != Leading {
		t.Errorf("position = %v, want Leading", got[0].Position)
	}
	if got[0].Text != "# doc" {
		t.Errorf("text = %q", got[0].Text)
	}
}

func TestAttach_LeadingBlock(t *testing.T) {
	src := "# one\n# two\n# three\ndef f():\n    pass\n"
	f, tr := parseTrivia(t, src)
	stmt := firstStmt(t, f)
	got := tr.ByNode[stmt]
	if len(got) != 3 {
		t.Fatalf("expected 3 leading comments, got %d (%v)", len(got), got)
	}
	want := []string{"# one", "# two", "# three"}
	for i, c := range got {
		if c.Position != Leading {
			t.Errorf("[%d] position = %v, want Leading", i, c.Position)
		}
		if c.Text != want[i] {
			t.Errorf("[%d] text = %q, want %q", i, c.Text, want[i])
		}
	}
}

func TestAttach_NestedTrailing(t *testing.T) {
	// The trailing comment must attach to the inner Return, not the
	// outer FunctionDef.
	src := "def f():\n    return 1  # ret\n"
	f, tr := parseTrivia(t, src)
	fn, ok := firstStmt(t, f).(*ast.FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef, got %T", firstStmt(t, f))
	}
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 body stmt, got %d", len(fn.Body))
	}
	ret := fn.Body[0]
	if comments := tr.ByNode[fn]; len(comments) != 0 {
		t.Errorf("FunctionDef should have no trivia, got %v", comments)
	}
	got := tr.ByNode[ret]
	if len(got) != 1 || got[0].Position != Trailing || got[0].Text != "# ret" {
		t.Errorf("ret trivia = %v, want one Trailing '# ret'", got)
	}
}

func TestAttach_EOFOrphan(t *testing.T) {
	// A comment with no following statement lands in Trivia.File.
	src := "x = 1\n# bye\n"
	_, tr := parseTrivia(t, src)
	if len(tr.File) != 1 {
		t.Fatalf("expected 1 file-level comment, got %v", tr.File)
	}
	if tr.File[0].Text != "# bye" {
		t.Errorf("text = %q", tr.File[0].Text)
	}
	if tr.File[0].Position != Leading {
		t.Errorf("position = %v, want Leading", tr.File[0].Position)
	}
}

func TestAttach_TypeComment(t *testing.T) {
	// TYPE_COMMENT follows the same trailing rule.
	src := "x = 1  # type: int\n"
	f, tr := parseTrivia(t, src)
	stmt := firstStmt(t, f)
	got := tr.ByNode[stmt]
	if len(got) != 1 {
		t.Fatalf("expected 1 comment, got %d (%v)", len(got), got)
	}
	if got[0].Text != "# type: int" {
		t.Errorf("text = %q", got[0].Text)
	}
	if got[0].Position != Trailing {
		t.Errorf("position = %v, want Trailing", got[0].Position)
	}
}

func TestAttach_OnlyComments(t *testing.T) {
	// A file with no statements parks every comment in Trivia.File.
	src := "# alpha\n# beta\n"
	_, tr := parseTrivia(t, src)
	if len(tr.ByNode) != 0 {
		t.Errorf("ByNode should be empty, got %v", tr.ByNode)
	}
	if len(tr.File) != 2 {
		t.Fatalf("expected 2 file-level comments, got %v", tr.File)
	}
	got := []string{tr.File[0].Text, tr.File[1].Text}
	want := []string{"# alpha", "# beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("texts = %v, want %v", got, want)
	}
}

func TestAttach_NoComments(t *testing.T) {
	// No comments at all: both fields are empty (and the map is
	// never nil, so callers can index without a guard).
	_, tr := parseTrivia(t, "x = 1\ny = 2\n")
	if len(tr.ByNode) != 0 {
		t.Errorf("ByNode should be empty, got %v", tr.ByNode)
	}
	if len(tr.File) != 0 {
		t.Errorf("File should be empty, got %v", tr.File)
	}
}

func TestAttach_FreshPerCall(t *testing.T) {
	// Two calls return distinct maps — mutating one shouldn't affect
	// the other (no caching).
	f, _ := parseTrivia(t, "x = 1  # tail\n")
	a := f.AttachComments()
	b := f.AttachComments()
	if &a.ByNode == &b.ByNode {
		t.Fatal("AttachComments returned the same map twice")
	}
	stmt := firstStmt(t, f)
	a.ByNode[stmt] = nil
	if len(b.ByNode[stmt]) == 0 {
		t.Errorf("mutation of one Trivia leaked into another")
	}
}
