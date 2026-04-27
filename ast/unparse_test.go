package ast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/gopapy/legacy/parser"
)

// roundTrip parses src, unparses, re-parses, and returns the two Dump
// strings for comparison.
func roundTrip(t *testing.T, name, src string) (string, string) {
	t.Helper()
	f1, err := parser.ParseString(name, src)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	mod1 := FromFile(f1)
	want := Dump(mod1)
	out := Unparse(mod1)
	f2, err := parser.ParseString(name+".reparse", out)
	if err != nil {
		t.Fatalf("reparse %s: %v\nunparsed:\n%s", name, err, out)
	}
	got := Dump(FromFile(f2))
	return want, got
}

func TestUnparse_RoundTrip_Fixtures(t *testing.T) {
	// tests/grammar lives two directories up from the package source.
	matches, err := filepath.Glob("../tests/grammar/*.py")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Skip("no fixtures found")
	}
	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			want, got := roundTrip(t, path, string(data))
			if want != got {
				t.Fatalf("Dump mismatch\n want: %s\n  got: %s", want, got)
			}
		})
	}
}

func TestUnparse_RoundTrip_Snippets(t *testing.T) {
	cases := []string{
		"x = 1\n",
		"a + b * c\n",
		"(a + b) * c\n",
		"1 + 2 + 3\n",
		"2 ** 3 ** 4\n",
		"-x ** 2\n",
		"a if b else c\n",
		"a or b and c\n",
		"not a == b\n",
		"a < b < c\n",
		"f(1, 2, *xs, **kws)\n",
		"a[1:2:3]\n",
		"[x for x in xs if x]\n",
		"{k: v for k, v in d.items()}\n",
		"lambda x, y=1: x + y\n",
		"(x := 1) + 2\n",
		"f'hello {name!r}'\n",
		"f'{x:>{w}.{p}f}'\n",
	}
	for _, src := range cases {
		t.Run(strings.TrimSpace(src), func(t *testing.T) {
			want, got := roundTrip(t, "<snippet>", src)
			if want != got {
				t.Fatalf("Dump mismatch\n want: %s\n  got: %s", want, got)
			}
		})
	}
}
