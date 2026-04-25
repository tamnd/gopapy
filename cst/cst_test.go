package cst

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/gopapy/v1/lex"
)

func TestParseRoundTrip(t *testing.T) {
	src := []byte("x = 1  # inline comment\n# leading comment\ny = 2\n")
	f, err := Parse("snippet.py", src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !bytes.Equal(f.Source(), src) {
		t.Fatalf("Source not byte-equal:\nwant=%q\ngot =%q", src, f.Source())
	}
	if f.AST == nil {
		t.Fatal("AST is nil")
	}
	// The token stream must contain at least the two comments. The
	// parser path drops them; the cst path keeps them.
	var comments int
	for _, tk := range f.Tokens() {
		if tk.Kind == lex.COMMENT {
			comments++
		}
	}
	if comments != 2 {
		t.Fatalf("expected 2 COMMENT tokens, got %d", comments)
	}
}

func TestSourceCloneIsolated(t *testing.T) {
	src := []byte("pass\n")
	f, err := Parse("x.py", src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	src[0] = 'X'
	if f.Source()[0] != 'p' {
		t.Fatalf("File.Source() reflects caller mutation: %q", f.Source())
	}
}

func TestRoundTripGrammarFixtures(t *testing.T) {
	dir := filepath.Join("..", "tests", "grammar")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			f, err := Parse(name, src)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !bytes.Equal(f.Source(), src) {
				t.Fatalf("Source() not byte-equal to input")
			}
		})
	}
}

// TestTokenCoverage asserts that the union of token byte spans plus
// the gaps between consecutive tokens covers every byte of the
// source. A gap is whitespace; the test verifies that those bytes
// are indeed whitespace, not lost content.
func TestTokenCoverage(t *testing.T) {
	dir := filepath.Join("..", "tests", "grammar")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			f, err := Parse(name, src)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			cursor := 0
			for _, tk := range f.Tokens() {
				switch tk.Kind {
				case lex.INDENT, lex.DEDENT, lex.ENDMARKER:
					// Synthetic tokens with zero source span; they
					// don't advance the cursor.
					continue
				}
				start := tk.Pos.Offset
				end := tk.End.Offset
				if start < cursor {
					t.Fatalf("token %v overlaps cursor %d", tk, cursor)
				}
				gap := src[cursor:start]
				for _, b := range gap {
					if b != ' ' && b != '\t' && b != '\n' && b != '\\' && b != '\r' {
						t.Fatalf("non-whitespace byte %q in gap [%d:%d] before %v", b, cursor, start, tk)
					}
				}
				if end > cursor {
					cursor = end
				}
			}
			// The remaining tail (trailing newline, etc.) must be
			// whitespace too.
			for _, b := range src[cursor:] {
				if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
					t.Fatalf("non-whitespace tail byte %q at offset %d", b, cursor)
				}
			}
		})
	}
}
