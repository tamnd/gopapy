package lex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzScan drives the scanner over arbitrary bytes and asserts it
// terminates without panicking. The bound on iterations exists because
// the failure mode that motivated this fuzz target is the lexer
// returning a zero-width token at the same offset forever — without an
// iteration cap that bug looks like a hung test, not a fuzz crash.
func FuzzScan(f *testing.F) {
	seedFromGrammarFixtures(f)
	for _, s := range scannerSeeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		s := NewScanner(src, "fuzz.py")
		// 8x len(src)+64 is enough headroom for triple-quoted strings
		// and indent/dedent injection without letting a stuck scanner
		// run away.
		budget := 8*len(src) + 64
		for range budget {
			tok, err := s.Scan()
			if err != nil {
				return
			}
			if tok.Kind == EOF {
				return
			}
		}
		t.Fatalf("scanner did not terminate within %d steps; last pos=%d/%d", budget, s.pos, len(src))
	})
}

var scannerSeeds = []string{
	"",
	"\xef\xbb\xbf",                  // BOM only
	"\r",                            // bare CR
	"\r\n",                          // CRLF
	"\xef\xbb\xbf# comment\n",       // BOM + comment
	"€",                        // non-ident multi-byte rune (€)
	"\xff\xfe",                      // invalid UTF-8 lead bytes
	"\\\n",                          // line continuation at EOF
	"\"unterminated",                // unterminated string
	"\"\"\"unterminated triple",     // unterminated triple
	"f\"{",                          // unterminated f-string interpolation
	"f\"{a:{b:{c}}}\"",              // nested f-string format specs
	"# type: int\n",                 // type comment
	"\xc2",                          // truncated UTF-8 sequence
	strings.Repeat("(", 200),        // deeply unbalanced parens
	strings.Repeat(" ", 100) + "x",  // big leading indent
	"def f():\n\tpass\n",            // tab indent
	"if x:\n    pass\nelse:\n y\n", // mixed indents
}

// seedFromGrammarFixtures registers every .py under tests/grammar as a
// fuzz seed so the fuzzer starts from inputs that already exercise real
// grammar rather than wandering through random byte space.
func seedFromGrammarFixtures(f *testing.F) {
	f.Helper()
	dir := filepath.Join("..", "tests", "grammar")
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Not fatal — fuzz seeds are an optimisation; the hard-coded
		// scannerSeeds still drive the corpus.
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		f.Add(src)
	}
}
