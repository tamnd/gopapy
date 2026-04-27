package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixtures(b *testing.B) []struct {
	name string
	src  string
} {
	b.Helper()
	matches, err := filepath.Glob("../tests/grammar/*.py")
	if err != nil || len(matches) == 0 {
		b.Fatalf("glob: %v", err)
	}
	out := make([]struct {
		name string
		src  string
	}, 0, len(matches))
	var total int
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil {
			b.Fatalf("read %s: %v", p, err)
		}
		out = append(out, struct {
			name string
			src  string
		}{filepath.Base(p), string(data)})
		total += len(data)
	}
	b.SetBytes(int64(total))
	return out
}

func BenchmarkParseFixtures(b *testing.B) {
	fixtures := loadFixtures(b)
	b.ResetTimer()
	for b.Loop() {
		for _, f := range fixtures {
			if _, err := ParseString(f.name, f.src); err != nil {
				b.Fatalf("%s: %v", f.name, err)
			}
		}
	}
}
