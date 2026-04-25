package lex

import (
	"os"
	"path/filepath"
	"testing"
)

// loadFixtures reads every .py file under tests/grammar into one slice
// of (name, bytes). Benchmarks reuse this so the read cost doesn't show
// up on the benchmark line.
func loadFixtures(b *testing.B) []struct {
	name string
	src  []byte
} {
	b.Helper()
	matches, err := filepath.Glob("../tests/grammar/*.py")
	if err != nil || len(matches) == 0 {
		b.Fatalf("glob: %v", err)
	}
	out := make([]struct {
		name string
		src  []byte
	}, 0, len(matches))
	var total int
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil {
			b.Fatalf("read %s: %v", p, err)
		}
		out = append(out, struct {
			name string
			src  []byte
		}{filepath.Base(p), data})
		total += len(data)
	}
	b.SetBytes(int64(total))
	return out
}

// BenchmarkScanFixtures times a Scanner-only walk over every grammar
// fixture. SetBytes(total fixture size) reports MB/s so the number
// survives hardware moves.
func BenchmarkScanFixtures(b *testing.B) {
	fixtures := loadFixtures(b)
	b.ResetTimer()
	for b.Loop() {
		for _, f := range fixtures {
			s := NewScanner(f.src, f.name)
			for {
				t, err := s.Scan()
				if err != nil {
					b.Fatalf("%s: %v", f.name, err)
				}
				if t.Kind == EOF {
					break
				}
			}
		}
	}
}

// BenchmarkIndentFixtures times the full logical token stream
// (Scanner + Indent), which is what the parser actually sees.
func BenchmarkIndentFixtures(b *testing.B) {
	fixtures := loadFixtures(b)
	b.ResetTimer()
	for b.Loop() {
		for _, f := range fixtures {
			it := NewIndent(NewScanner(f.src, f.name))
			for {
				t, err := it.Next()
				if err != nil {
					b.Fatalf("%s: %v", f.name, err)
				}
				if t.Kind == ENDMARKER {
					break
				}
			}
		}
	}
}
