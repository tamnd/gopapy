package linter

import (
	"strings"
	"testing"

	"github.com/tamnd/gopapy/v1/ast"
	"github.com/tamnd/gopapy/v1/parser"
)

// fixSrc parses src, runs Fix, then unparses. Returns the resulting
// source plus the codes of fixed diagnostics in stable order.
func fixSrc(t *testing.T, src string) (string, []string) {
	t.Helper()
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	mod := ast.FromFile(f)
	mod, fixed := Fix(mod)
	codes := make([]string, 0, len(fixed))
	for _, fd := range fixed {
		codes = append(codes, fd.Code)
	}
	return ast.Unparse(mod), codes
}

func TestFixF401(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		want      string
		fixCount  int
	}{
		{
			name:     "single-unused-import",
			src:      "import os\n",
			want:     "",
			fixCount: 1,
		},
		{
			name:     "single-used-import",
			src:      "import os\nprint(os)\n",
			want:     "import os\nprint(os)\n",
			fixCount: 0,
		},
		{
			name:     "comma-import-one-unused",
			src:      "import os, sys\nprint(sys)\n",
			want:     "import sys\nprint(sys)\n",
			fixCount: 1,
		},
		{
			name:     "comma-import-all-unused",
			src:      "import os, sys\n",
			want:     "",
			fixCount: 2,
		},
		{
			name:     "from-import-one-unused",
			src:      "from m import x, y\nprint(y)\n",
			want:     "from m import y\nprint(y)\n",
			fixCount: 1,
		},
		{
			name:     "from-import-all-unused",
			src:      "from m import x, y\n",
			want:     "",
			fixCount: 2,
		},
		{
			name:     "from-import-as-unused",
			src:      "from m import x as z\n",
			want:     "",
			fixCount: 1,
		},
		{
			name:     "from-import-as-used",
			src:      "from m import x as z\nprint(z)\n",
			want:     "from m import x as z\nprint(z)\n",
			fixCount: 0,
		},
		{
			name:     "star-import-untouched",
			src:      "from m import *\n",
			want:     "from m import *\n",
			fixCount: 0,
		},
		{
			name:     "future-import-untouched",
			src:      "from __future__ import annotations\n",
			want:     "from __future__ import annotations\n",
			fixCount: 0,
		},
		{
			name:     "dotted-unused",
			src:      "import a.b.c\n",
			want:     "",
			fixCount: 1,
		},
		{
			name:     "dotted-used-via-top",
			src:      "import a.b.c\nprint(a.b.c)\n",
			want:     "import a.b.c\nprint(a.b.c)\n",
			fixCount: 0,
		},
		{
			name:     "function-scope-import-untouched",
			src:      "def f():\n    import os\n",
			want:     "def f():\n    import os\n",
			fixCount: 0,
		},
		{
			name:     "if-block-import-fixed",
			src:      "if True:\n    import os\n    import sys\nprint(sys)\n",
			want:     "if True:\n    import sys\nprint(sys)\n",
			fixCount: 1,
		},
		{
			name:     "try-block-import-fixed",
			src:      "try:\n    import json\nexcept ImportError:\n    import simplejson as json\nprint(json)\n",
			want:     "try:\n    import json\nexcept ImportError:\n    import simplejson as json\nprint(json)\n",
			fixCount: 0,
		},
		{
			name:     "class-body-import-untouched",
			src:      "class C:\n    import os\n",
			want:     "class C:\n    import os\n",
			fixCount: 0,
		},
		{
			name:     "mixed-stmts-around-imports",
			src:      "import os\nx = 1\nimport sys\nprint(x, sys)\n",
			want:     "x = 1\nimport sys\nprint(x, sys)\n",
			fixCount: 1,
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
				if c != "F401" {
					t.Errorf("expected only F401 fixes, got %q", c)
				}
			}
		})
	}
}

func TestFixF811DeadStoreLiteral(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		want      string
		fixCount  int
		fixedCode string
	}{
		{
			name:      "literal-then-rebind",
			src:       "def f():\n    x = 1\n    x = 2\n    return x\n",
			want:      "def f():\n    x = 2\n    return x\n",
			fixCount:  1,
			fixedCode: "F811",
		},
		{
			name:      "literal-then-rebind-string",
			src:       "def f():\n    s = ''\n    s = 'hi'\n    return s\n",
			want:      "def f():\n    s = 'hi'\n    return s\n",
			fixCount:  1,
			fixedCode: "F811",
		},
		{
			name:      "non-literal-rhs-untouched",
			src:       "def f():\n    x = expensive()\n    x = 2\n    return x\n",
			want:      "def f():\n    x = expensive()\n    x = 2\n    return x\n",
			fixCount:  0,
			fixedCode: "",
		},
		{
			name:      "non-adjacent-untouched",
			src:       "def f():\n    x = 1\n    y = 2\n    x = 3\n    return x\n",
			want:      "def f():\n    x = 1\n    y = 2\n    x = 3\n    return x\n",
			fixCount:  0,
			fixedCode: "",
		},
		{
			name:      "annassign-rebind",
			src:       "def f():\n    x = 1\n    x: int = 2\n    return x\n",
			want:      "def f():\n    x: int = 2\n    return x\n",
			fixCount:  1,
			fixedCode: "F811",
		},
		{
			name:      "augassign-keeps-store",
			src:       "def f():\n    x = 1\n    x += 1\n    return x\n",
			want:      "def f():\n    x = 1\n    x += 1\n    return x\n",
			fixCount:  0,
			fixedCode: "",
		},
		{
			name:      "module-scope-untouched",
			src:       "x = 1\nx = 2\nprint(x)\n",
			want:      "x = 1\nx = 2\nprint(x)\n",
			fixCount:  0,
			fixedCode: "",
		},
		{
			name:      "method-body-applies",
			src:       "class C:\n    def m(self):\n        x = 1\n        x = 2\n        return x\n",
			want:      "class C:\n    def m(self):\n        x = 2\n        return x\n",
			fixCount:  1,
			fixedCode: "F811",
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
				if c != tc.fixedCode {
					t.Errorf("expected only %q fixes, got %q", tc.fixedCode, c)
				}
			}
		})
	}
}

func TestFixF811NoRegressionAfterFix(t *testing.T) {
	// After the dead-store fix, a re-lint must report zero F811 for
	// the inputs that had it.
	src := "def f():\n    x = 1\n    x = 2\n    return x\n"
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod := ast.FromFile(f)
	Fix(mod)
	diags := Lint(mod)
	for _, d := range diags {
		if d.Code == "F811" {
			t.Errorf("Fix left an F811 standing: %s", d.String())
		}
	}
}

func TestFixIdempotent(t *testing.T) {
	// After a Fix, re-linting should report zero F401s on the result.
	src := "import os, sys\nfrom m import x, y\nfrom __future__ import annotations\nprint(sys, y)\n"
	got, _ := fixSrc(t, src)
	want := "import sys\nfrom m import y\nfrom __future__ import annotations\nprint(sys, y)\n"
	if got != want {
		t.Errorf("first fix mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// Second pass should be a no-op.
	got2, codes := fixSrc(t, got)
	if got2 != got {
		t.Errorf("second fix changed the source\n--- got2 ---\n%s\n--- got ---\n%s", got2, got)
	}
	if len(codes) != 0 {
		t.Errorf("second fix reported %d fixes, want 0", len(codes))
	}
}

func TestFixThenLintNoF401(t *testing.T) {
	src := "import os\nimport sys\nfrom m import x, y\nprint(sys, y)\n"
	f, err := parser.ParseString("<test>", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod := ast.FromFile(f)
	Fix(mod)
	diags := Lint(mod)
	for _, d := range diags {
		if d.Code == "F401" {
			t.Errorf("Fix left an F401 standing: %s", d.String())
		}
	}
}

func TestFixedDiagnosticPos(t *testing.T) {
	// The Pos on a FixedDiagnostic should point at the original import
	// statement so editors can show "fixed here" markers.
	src := "import os\nimport sys\nprint(sys)\n"
	f, _ := parser.ParseString("<test>", src)
	mod := ast.FromFile(f)
	_, fixed := Fix(mod)
	if len(fixed) != 1 {
		t.Fatalf("got %d fixed, want 1", len(fixed))
	}
	if fixed[0].Pos.Lineno != 1 {
		t.Errorf("Pos.Lineno = %d, want 1", fixed[0].Pos.Lineno)
	}
	if !strings.Contains(fixed[0].Msg, "'os'") {
		t.Errorf("Msg = %q, want to contain 'os'", fixed[0].Msg)
	}
}

func TestFixNilSafe(t *testing.T) {
	got, fixed := Fix(nil)
	if got != nil || fixed != nil {
		t.Errorf("Fix(nil) = (%v, %v), want (nil, nil)", got, fixed)
	}
}
