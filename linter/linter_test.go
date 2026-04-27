package linter

import (
	"strings"
	"testing"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/parser"
)

// lintSrc is the test helper. It mirrors LintFile (parses + lints +
// applies # noqa) but skips disk I/O so fixtures stay inline.
func lintSrc(t *testing.T, src string) []string {
	t.Helper()
	got, err := LintFile("<test>", []byte(src))
	if err != nil {
		t.Fatalf("LintFile: %v", err)
	}
	out := make([]string, 0, len(got))
	for _, d := range got {
		out = append(out, d.Code)
	}
	return out
}

// lintModule parses + lints without # noqa, mirroring Lint(mod) so we
// can verify that path independently of the trivia layer.
func lintModule(t *testing.T, src string) []string {
	t.Helper()
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	got := Lint(ast.FromFile(f))
	out := make([]string, 0, len(got))
	for _, d := range got {
		out = append(out, d.Code)
	}
	return out
}

func TestF401(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "unused-import",
			src:  "import os\n",
			want: []string{"F401"},
		},
		{
			name: "used-import",
			src:  "import os\nprint(os.getcwd())\n",
			want: nil,
		},
		{
			name: "unused-from-import",
			src:  "from os.path import join\n",
			want: []string{"F401"},
		},
		{
			name: "used-from-import",
			src:  "from os.path import join\nprint(join('a', 'b'))\n",
			want: nil,
		},
		{
			name: "alias-unused",
			src:  "import numpy as np\n",
			want: []string{"F401"},
		},
		{
			name: "alias-used",
			src:  "import numpy as np\nnp.array([1])\n",
			want: nil,
		},
		{
			name: "star-import-not-flagged",
			src:  "from os import *\n",
			want: nil,
		},
		{
			name: "function-scope-import-not-flagged",
			src:  "def f():\n    import os\n",
			want: nil,
		},
		{
			name: "noqa-suppresses",
			src:  "import os  # noqa: F401\n",
			want: nil,
		},
		{
			name: "noqa-bare-suppresses",
			src:  "import os  # noqa\n",
			want: nil,
		},
		{
			name: "noqa-other-code-does-not-suppress",
			src:  "import os  # noqa: F841\n",
			want: []string{"F401"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintSrc(t, tc.src)
			if !equalStrings(got, tc.want) {
				t.Errorf("got %v, want %v\nsrc:\n%s", got, tc.want, tc.src)
			}
		})
	}
}

func TestF541(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "no-placeholder",
			src:  "x = f\"hello\"\n",
			want: []string{"F541"},
		},
		{
			name: "empty-fstring",
			src:  "x = f\"\"\n",
			want: []string{"F541"},
		},
		{
			name: "single-quote-no-placeholder",
			src:  "x = f'hi'\n",
			want: []string{"F541"},
		},
		{
			name: "with-placeholder",
			src:  "y = 1\nx = f\"y={y}\"\n",
			want: nil,
		},
		{
			name: "plain-string-not-flagged",
			src:  "x = \"hello\"\n",
			want: nil,
		},
		{
			name: "concat-with-placeholder",
			src:  "y = 1\nx = f\"prefix {y}\"\n",
			want: nil,
		},
		{
			name: "nested-fstring-inner-flagged",
			src:  "y = f\"outer {f'static'}\"\n",
			want: []string{"F541"},
		},
		{
			name: "noqa-suppresses",
			src:  "x = f\"hello\"  # noqa: F541\n",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintSrc(t, tc.src)
			if !equalStrings(got, tc.want) {
				t.Errorf("got %v, want %v\nsrc:\n%s", got, tc.want, tc.src)
			}
		})
	}
}

func TestF632(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "is-int",
			src:  "x = 1\nif x is 1: pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-str",
			src:  "x = 'a'\nif x is 'a': pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-bool",
			src:  "x = True\nif x is True: pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-not-int",
			src:  "x = 1\nif x is not 2: pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-tuple-literal",
			src:  "x = ()\nif x is (1, 2): pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-negative-int",
			src:  "x = 0\nif x is -1: pass\n",
			want: []string{"F632"},
		},
		{
			name: "is-none-allowed",
			src:  "x = None\nif x is None: pass\n",
			want: nil,
		},
		{
			name: "is-not-none-allowed",
			src:  "x = 1\nif x is not None: pass\n",
			want: nil,
		},
		{
			name: "is-name-allowed",
			src:  "x = 1\ny = 1\nif x is y: pass\n",
			want: nil,
		},
		{
			name: "eq-literal-not-flagged",
			src:  "x = 1\nif x == 1: pass\n",
			want: nil,
		},
		{
			name: "chain-mixed",
			src:  "x = 1\nif x is 1 is None: pass\n",
			want: []string{"F632"},
		},
		{
			name: "noqa-suppresses",
			src:  "x = 1\nif x is 1: pass  # noqa: F632\n",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintSrc(t, tc.src)
			if !equalStrings(got, tc.want) {
				t.Errorf("got %v, want %v\nsrc:\n%s", got, tc.want, tc.src)
			}
		})
	}
}

func TestF811(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "double-def",
			src:  "def f(): pass\ndef f(): pass\n",
			want: []string{"F811"},
		},
		{
			name: "double-def-with-use-between",
			src:  "def f(): pass\nf()\ndef f(): pass\n",
			want: nil,
		},
		{
			name: "double-import",
			src:  "import os\nimport os\nos.getcwd()\n",
			want: []string{"F811"},
		},
		{
			name: "dead-store",
			src:  "def g():\n    x = 1\n    x = 2\n    return x\n",
			want: []string{"F811"},
		},
		{
			name: "no-dead-store-with-read",
			src:  "def g():\n    x = 1\n    print(x)\n    x = 2\n    return x\n",
			want: nil,
		},
		{
			name: "param-then-assign-not-flagged",
			src:  "def g(x):\n    x = 1\n    return x\n",
			want: nil,
		},
		{
			name: "method-redef",
			src:  "class C:\n    def m(self): pass\n    def m(self): pass\n",
			want: []string{"F811"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintSrc(t, tc.src)
			if !equalStrings(got, tc.want) {
				t.Errorf("got %v, want %v\nsrc:\n%s", got, tc.want, tc.src)
			}
		})
	}
}

func TestF841(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "unused-local",
			src:  "def f():\n    x = 1\n",
			want: []string{"F841"},
		},
		{
			name: "used-local",
			src:  "def f():\n    x = 1\n    return x\n",
			want: nil,
		},
		{
			name: "module-scope-not-flagged",
			src:  "x = 1\n",
			want: nil,
		},
		{
			name: "underscore-not-flagged",
			src:  "def f():\n    _ = 1\n",
			want: nil,
		},
		{
			name: "for-target-not-flagged",
			src:  "def f(xs):\n    for x in xs:\n        print(1)\n",
			want: nil,
		},
		{
			name: "with-target-not-flagged",
			src:  "def f():\n    with open('x') as fh:\n        return 1\n",
			want: nil,
		},
		{
			name: "except-target-not-flagged",
			src:  "def f():\n    try:\n        pass\n    except Exception as e:\n        return 1\n",
			want: nil,
		},
		{
			name: "augassign-not-flagged",
			src:  "def f():\n    x = 1\n    x += 1\n    return x\n",
			want: nil,
		},
		{
			name: "annassign-no-value-not-flagged",
			src:  "def f():\n    x: int\n    return 1\n",
			want: nil,
		},
		{
			name: "noqa-suppresses",
			src:  "def f():\n    x = 1  # noqa: F841\n",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lintSrc(t, tc.src)
			if !equalStrings(got, tc.want) {
				t.Errorf("got %v, want %v\nsrc:\n%s", got, tc.want, tc.src)
			}
		})
	}
}

func TestLintWithoutNoqa(t *testing.T) {
	// Lint(mod) ignores comments, so noqa does not suppress.
	got := lintModule(t, "import os  # noqa\n")
	if len(got) != 1 || got[0] != "F401" {
		t.Errorf("Lint() should not honor noqa; got %v", got)
	}
}

func TestDiagnosticFormat(t *testing.T) {
	// One concrete check that the position and message both look right.
	diags, err := LintFile("a.py", []byte("import os\n"))
	if err != nil {
		t.Fatalf("LintFile: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d", len(diags))
	}
	d := diags[0]
	if d.Filename != "a.py" {
		t.Errorf("Filename = %q, want a.py", d.Filename)
	}
	if d.Pos.Lineno != 1 {
		t.Errorf("Pos.Lineno = %d, want 1", d.Pos.Lineno)
	}
	if d.Code != "F401" {
		t.Errorf("Code = %q, want F401", d.Code)
	}
	want := "'os' imported but unused"
	if !strings.Contains(d.Msg, want) {
		t.Errorf("Msg = %q, want substring %q", d.Msg, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
