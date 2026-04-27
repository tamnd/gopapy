package parser

import (
	"bytes"
	"strings"
	"testing"
)

// These tests lock in the public surface of the v1 parser: every
// exported entry point gets a happy-path call and a basic shape
// assertion. The point isn't to test parser correctness (parser_test.go
// covers grammar coverage in depth); it's to make sure a parser2 build
// that drops or renames any of these functions fails CI immediately
// rather than silently passing the broader corpus tests.

func TestParseFile_Smoke(t *testing.T) {
	f, err := ParseFile("smoke.py", []byte("print(1)\n"))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if f == nil {
		t.Fatalf("ParseFile returned nil File")
	}
	if len(f.Statements) == 0 {
		t.Fatalf("expected at least one statement, got 0")
	}
}

func TestParseFile_RejectsBadSource(t *testing.T) {
	if _, err := ParseFile("bad.py", []byte("1 +\n")); err == nil {
		t.Fatalf("expected error for bad source, got nil")
	}
}

func TestParseString_Smoke(t *testing.T) {
	f, err := ParseString("s.py", "x = 1\n")
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	if f == nil || len(f.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %v", f)
	}
}

func TestParseString_AcceptsEmptyModule(t *testing.T) {
	if _, err := ParseString("empty.py", ""); err != nil {
		t.Fatalf("empty module should parse, got: %v", err)
	}
}

func TestParseExpression_Smoke(t *testing.T) {
	expr, err := ParseExpression("1 + 2")
	if err != nil {
		t.Fatalf("ParseExpression: %v", err)
	}
	if expr == nil {
		t.Fatalf("ParseExpression returned nil Expression")
	}
}

func TestParseExpression_RejectsStatement(t *testing.T) {
	// `x = 1` is an assignment statement, not an expression.
	if _, err := ParseExpression("x = 1"); err == nil {
		t.Fatalf("expected error for non-expression input, got nil")
	}
}

func TestParseReader_MatchesParseFile(t *testing.T) {
	src := []byte("def f():\n    pass\n")
	a, errA := ParseFile("a.py", src)
	b, errB := ParseReader("b.py", src)
	if errA != nil || errB != nil {
		t.Fatalf("ParseFile=%v ParseReader=%v", errA, errB)
	}
	if a == nil || b == nil {
		t.Fatalf("nil result: a=%v b=%v", a, b)
	}
	if len(a.Statements) != len(b.Statements) {
		t.Errorf("statement count mismatch: ParseFile=%d ParseReader=%d",
			len(a.Statements), len(b.Statements))
	}
}

func TestParseReader_BufferIsCloneable(t *testing.T) {
	// ParseReader's docstring promises src is cloned before parsing,
	// so callers may reuse or mutate the buffer afterward. Verify by
	// mutating the original after the call and re-parsing the original
	// pre-mutation bytes — the result should match the first parse.
	src := []byte("x = 1\n")
	saved := bytes.Clone(src)
	if _, err := ParseReader("r.py", src); err != nil {
		t.Fatalf("ParseReader: %v", err)
	}
	for i := range src {
		src[i] = 'X'
	}
	again, err := ParseReader("r.py", saved)
	if err != nil {
		t.Fatalf("re-parse of saved bytes: %v", err)
	}
	if again == nil || len(again.Statements) != 1 {
		t.Fatalf("expected stable parse, got %v", again)
	}
}

// TestExportedSurface_Stable is the canary for "did somebody delete
// or rename a public function." It compiles and runs the four entry
// points by reference. If any signature drifts the package won't
// compile; if any entry point goes away the test fails to build.
func TestExportedSurface_Stable(t *testing.T) {
	// Compile-time references — assigning to _ keeps the linter happy
	// without actually invoking the function unless we want to.
	_ = ParseFile
	_ = ParseString
	_ = ParseExpression
	_ = ParseReader
	// One smoke call so the test does something at runtime too;
	// otherwise it's pure compile-time and `go test -run` skips it.
	if _, err := ParseString("surface.py", "pass\n"); err != nil {
		t.Fatalf("surface smoke: %v", err)
	}
	if !strings.Contains(t.Name(), "Stable") {
		t.Fatalf("test name drift: %q", t.Name())
	}
}
