package linter

import (
	"strings"
	"testing"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/parser"
)

func TestFixE711(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		want     string
		fixCount int
	}{
		{
			name:     "eq-none-rhs",
			src:      "if x == None:\n    pass\n",
			want:     "if x is None:\n    pass\n",
			fixCount: 1,
		},
		{
			name:     "noteq-none-rhs",
			src:      "if x != None:\n    pass\n",
			want:     "if x is not None:\n    pass\n",
			fixCount: 1,
		},
		{
			name:     "eq-none-lhs",
			src:      "if None == x:\n    pass\n",
			want:     "if None is x:\n    pass\n",
			fixCount: 1,
		},
		{
			name:     "is-none-untouched",
			src:      "if x is None:\n    pass\n",
			want:     "if x is None:\n    pass\n",
			fixCount: 0,
		},
		{
			name:     "non-none-eq-untouched",
			src:      "if x == 0:\n    pass\n",
			want:     "if x == 0:\n    pass\n",
			fixCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, codes := fixSrc(t, tc.src)
			if got != tc.want {
				t.Errorf("source mismatch\n--- got ---\n%s\n--- want ---\n%s", got, tc.want)
			}
			if len(codes) != tc.fixCount {
				t.Errorf("fixCount = %d, want %d (codes: %v)", len(codes), tc.fixCount, codes)
			}
			for _, c := range codes {
				if c != "E711" {
					t.Errorf("expected only E711 fixes, got %q", c)
				}
			}
		})
	}
}

func TestFixE711PerFileIgnore(t *testing.T) {
	src := "if x == None:\n    pass\n"
	f, err := parser.ParseString("tests/foo.py", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod := ast.FromFile(f)
	cfg := Config{
		PerFile: map[string][]string{"tests/*": {"E711"}},
	}
	mod, fixed := FixWithConfig(mod, cfg, "tests/foo.py")
	if len(fixed) != 0 {
		t.Errorf("expected 0 fixes for ignored file, got %d", len(fixed))
	}
	got := ast.Unparse(mod)
	if !strings.Contains(got, "x == None") {
		t.Errorf("ignored file should still contain `x == None`, got:\n%s", got)
	}
}

func TestFixE711NoRegression(t *testing.T) {
	src := "if x == None:\n    pass\nif x != None:\n    pass\n"
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod := ast.FromFile(f)
	Fix(mod)
	for _, d := range Lint(mod) {
		if d.Code == "E711" {
			t.Errorf("Fix left an E711 standing: %s", d.String())
		}
	}
}
