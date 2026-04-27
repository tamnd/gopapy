package symbols

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/legacy/parser"
)

func BenchmarkBuildFixtures(b *testing.B) {
	matches, err := filepath.Glob("../tests/grammar/*.py")
	if err != nil || len(matches) == 0 {
		b.Fatalf("glob: %v", err)
	}
	type fixture struct {
		name string
		mod  *ast.Module
	}
	mods := make([]fixture, 0, len(matches))
	var total int
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil {
			b.Fatalf("read: %v", err)
		}
		f, err := parser.ParseString(p, string(data))
		if err != nil {
			b.Fatalf("parse %s: %v", p, err)
		}
		mods = append(mods, fixture{filepath.Base(p), ast.FromFile(f)})
		total += len(data)
	}
	b.SetBytes(int64(total))
	b.ResetTimer()
	for b.Loop() {
		for _, m := range mods {
			_ = Build(m.mod)
		}
	}
}
