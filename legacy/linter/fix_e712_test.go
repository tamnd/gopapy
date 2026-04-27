package linter

import (
	"testing"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/legacy/parser"
)

func TestFixE712(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		want     string
		fixCount int
	}{
		{
			name:     "eq-true",
			src:      "if x == True:\n    pass\n",
			want:     "if x is True:\n    pass\n",
			fixCount: 1,
		},
		{
			name:     "noteq-true",
			src:      "if x != True:\n    pass\n",
			want:     "if x is not True:\n    pass\n",
			fixCount: 1,
		},
		{
			name:     "eq-false",
			src:      "if x == False:\n    pass\n",
			want:     "if x is False:\n    pass\n",
			fixCount: 1,
		},
		{
			name:     "true-lhs",
			src:      "if True == x:\n    pass\n",
			want:     "if True is x:\n    pass\n",
			fixCount: 1,
		},
		{
			name:     "is-true-untouched",
			src:      "if x is True:\n    pass\n",
			want:     "if x is True:\n    pass\n",
			fixCount: 0,
		},
		{
			name:     "non-bool-eq-untouched",
			src:      "if x == 1:\n    pass\n",
			want:     "if x == 1:\n    pass\n",
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
				if c != "E712" {
					t.Errorf("expected only E712 fixes, got %q", c)
				}
			}
		})
	}
}

func TestFixE712NoRegression(t *testing.T) {
	src := "if x == True:\n    pass\nif x == False:\n    pass\n"
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod := ast.FromFile(f)
	Fix(mod)
	for _, d := range Lint(mod) {
		if d.Code == "E712" {
			t.Errorf("Fix left an E712 standing: %s", d.String())
		}
	}
}
