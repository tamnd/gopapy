package cst

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/gopapy/ast"
)

func unparseSrc(t *testing.T, src string) string {
	t.Helper()
	f, err := Parse("test.py", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return f.Unparse()
}

func TestUnparse_NoComments_MatchesAstUnparse(t *testing.T) {
	srcs := []string{
		"x = 1\n",
		"def f(a, b):\n    return a + b\n",
		"class C:\n    def m(self):\n        pass\n",
		"if x:\n    y = 1\nelse:\n    y = 2\n",
	}
	for _, src := range srcs {
		f, err := Parse("t.py", []byte(src))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		got := f.Unparse()
		want := ast.Unparse(f.AST)
		if got != want {
			t.Errorf("cst.Unparse diverged from ast.Unparse for src %q\ngot:  %q\nwant: %q", src, got, want)
		}
	}
}

func TestUnparse_TrailingInline(t *testing.T) {
	got := unparseSrc(t, "x = 1  # tail\n")
	want := "x = 1  # tail\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnparse_LeadingSingle(t *testing.T) {
	got := unparseSrc(t, "# doc\nx = 1\n")
	want := "# doc\nx = 1\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnparse_LeadingBlock(t *testing.T) {
	src := "# one\n# two\n# three\ndef f():\n    pass\n"
	got := unparseSrc(t, src)
	if got != src {
		t.Errorf("got %q, want %q", got, src)
	}
}

func TestUnparse_NestedTrailing(t *testing.T) {
	src := "def f():\n    return 1  # tag\n"
	got := unparseSrc(t, src)
	if got != src {
		t.Errorf("got %q, want %q", got, src)
	}
}

func TestUnparse_TypeComment(t *testing.T) {
	src := "x = 1  # type: int\n"
	got := unparseSrc(t, src)
	if got != src {
		t.Errorf("got %q, want %q", got, src)
	}
}

func TestUnparse_EOFOrphan(t *testing.T) {
	src := "x = 1\n# trailing module comment\n"
	got := unparseSrc(t, src)
	if got != src {
		t.Errorf("got %q, want %q", got, src)
	}
}

func TestUnparse_RoundTripGrammarFixtures(t *testing.T) {
	dir := filepath.Join("..", "tests", "grammar")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no grammar fixtures: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".py" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		f, err := Parse(path, src)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			continue
		}
		out := f.Unparse()
		f2, err := Parse(path, []byte(out))
		if err != nil {
			t.Errorf("reparse %s: %v\nunparsed:\n%s", path, err, out)
			continue
		}
		d1 := ast.Dump(f.AST)
		d2 := ast.Dump(f2.AST)
		if d1 != d2 {
			t.Errorf("dump diverged after cst round-trip for %s", path)
			continue
		}
		// Trivia counts must agree per host node.
		t1 := f.AttachComments()
		t2 := f2.AttachComments()
		if a, b := commentTotal(t1), commentTotal(t2); a != b {
			t.Errorf("trivia count mismatch for %s: orig=%d, round-trip=%d", path, a, b)
		}
	}
}

func commentTotal(t *Trivia) int {
	n := len(t.File)
	for _, cs := range t.ByNode {
		n += len(cs)
	}
	return n
}
