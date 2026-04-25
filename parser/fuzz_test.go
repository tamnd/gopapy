package parser

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pp "github.com/alecthomas/participle/v2"

	"github.com/tamnd/gopapy/v1/lex"
)

// FuzzParseFile asserts that ParseFile never panics and never returns
// an untyped error. Every error path must be either a participle.Error
// (grammar reject) or a *lex.Error (tokenizer reject); the rest signal
// the parser is leaking an internal failure.
func FuzzParseFile(f *testing.F) {
	seedParseFromFixtures(f)
	for _, s := range parserSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// Bound the source size; participle's lookahead can blow up
		// quadratically on truly massive inputs and that's not what
		// this fuzzer is hunting. The interesting bugs all surface in
		// inputs much smaller than this cap.
		if len(src) > 32*1024 {
			t.Skip()
		}
		_, err := ParseFile("fuzz.py", []byte(src))
		if err == nil {
			return
		}
		var pErr pp.Error
		var lErr *lex.Error
		if errors.As(err, &pErr) || errors.As(err, &lErr) {
			return
		}
		t.Fatalf("ParseFile returned untyped error: %T %v", err, err)
	})
}

var parserSeeds = []string{
	"",
	"\n",
	"pass\n",
	"x = 1\n",
	"def f():\n    pass\n",
	"f\"{x:{y}}\"\n",
	"match x:\n    case 1: pass\n",
	"async def f():\n    await g()\n",
	"@dec\nclass C(B): pass\n",
	"with (a as b, c as d):\n    pass\n",
	"a, *b, c = xs\n",
	"x: int = 1\n",
	"try:\n    f()\nexcept* E as e:\n    pass\n",
}

func seedParseFromFixtures(f *testing.F) {
	f.Helper()
	dir := filepath.Join("..", "tests", "grammar")
	entries, err := os.ReadDir(dir)
	if err != nil {
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
		f.Add(string(src))
	}
}
