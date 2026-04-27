package ast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/gopapy/legacy/parser"
)

// FuzzEmit asserts that FromFile and Unparse do not panic on any
// fuzz-derived parse tree. Strict round-trip equality is verified
// separately on the curated grammar fixtures (TestRoundTripFixtures);
// random byte input can produce participle trees with internally
// inconsistent shapes that the emitter then has to walk safely.
func FuzzEmit(f *testing.F) {
	dir := filepath.Join("..", "tests", "grammar")
	entries, err := os.ReadDir(dir)
	if err == nil {
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
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 16*1024 {
			t.Skip()
		}
		first, err := parser.ParseFile("fuzz.py", []byte(src))
		if err != nil {
			t.Skip()
		}
		mod := FromFile(first)
		emitted := Unparse(mod)
		// Re-parse the emitted source. Failure here is a genuine
		// emitter bug only when the round-trip happens to be supposed
		// to work; for random input we just want no panic.
		if second, err := parser.ParseFile("round.py", []byte(emitted)); err == nil {
			_ = FromFile(second)
		}
	})
}

// TestRoundTripFixtures pins the strict property — every curated
// grammar fixture must Dump-equal itself after a parse → unparse →
// parse cycle. The fuzz target gives up on this property because
// arbitrary bytes can parse to malformed shapes that the emitter
// can't faithfully reconstruct.
func TestRoundTripFixtures(t *testing.T) {
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
			first, err := parser.ParseFile(name, src)
			if err != nil {
				t.Fatalf("first parse: %v", err)
			}
			mod := FromFile(first)
			emitted := Unparse(mod)
			second, err := parser.ParseFile("round_"+name, []byte(emitted))
			if err != nil {
				t.Fatalf("round parse failed\n--- original ---\n%s\n--- unparse ---\n%s\n--- err ---\n%v", src, emitted, err)
			}
			got, want := Dump(FromFile(second)), Dump(mod)
			if got != want {
				t.Fatalf("round-trip Dump mismatch\n--- original ---\n%s\n--- unparse ---\n%s\n--- want ---\n%s\n--- got ---\n%s", src, emitted, want, got)
			}
		})
	}
}
