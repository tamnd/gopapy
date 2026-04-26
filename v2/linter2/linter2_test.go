package linter2_test

import (
	"strings"
	"testing"

	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/linter2"
	"github.com/tamnd/gopapy/v2/parser2"
)

func lint(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	mod, err := parser2.ParseFile("<test>", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return linter2.Lint(mod)
}

func lintFile(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	ds, err := linter2.LintFile("<test>", []byte(src))
	if err != nil {
		t.Fatalf("lintFile: %v", err)
	}
	return ds
}

func hasCode(ds []diag.Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}


// F401 — unused import

func TestF401UnusedImport(t *testing.T) {
	ds := lint(t, "import os\n")
	if !hasCode(ds, "F401") {
		t.Error("expected F401 for unused import os")
	}
}

func TestF401UsedImport(t *testing.T) {
	ds := lint(t, "import os\nprint(os.path)\n")
	if hasCode(ds, "F401") {
		t.Error("unexpected F401 for used import os")
	}
}

func TestF401FutureImport(t *testing.T) {
	ds := lint(t, "from __future__ import annotations\n")
	if hasCode(ds, "F401") {
		t.Error("F401 must not fire on __future__ imports")
	}
}

// F501 — %-format mismatch

func TestF501Mismatch(t *testing.T) {
	ds := lint(t, `"%s %s" % ("a",)` + "\n")
	if !hasCode(ds, "F501") {
		t.Error("expected F501 for format mismatch")
	}
}

func TestF501Match(t *testing.T) {
	ds := lint(t, `"%s" % "a"` + "\n")
	if hasCode(ds, "F501") {
		t.Error("unexpected F501 when arg count matches")
	}
}

// F541 — f-string without placeholders

func TestF541NoPlaceholder(t *testing.T) {
	ds := lint(t, "x = f\"hello\"\n")
	if !hasCode(ds, "F541") {
		t.Error("expected F541 for f-string without placeholders")
	}
}

func TestF541WithPlaceholder(t *testing.T) {
	ds := lint(t, "x = f\"hello {1}\"\n")
	if hasCode(ds, "F541") {
		t.Error("unexpected F541 when f-string has placeholder")
	}
}

// F632 — is with literal

func TestF632IsLiteral(t *testing.T) {
	ds := lint(t, "x is 1\n")
	if !hasCode(ds, "F632") {
		t.Error("expected F632 for `x is 1`")
	}
}

func TestF632IsNoneOk(t *testing.T) {
	ds := lint(t, "x is None\n")
	if hasCode(ds, "F632") {
		t.Error("unexpected F632 for `x is None`")
	}
}

// F811 — redefinition of unused

func TestF811Redefinition(t *testing.T) {
	ds := lint(t, "import os\nimport os\n")
	if !hasCode(ds, "F811") {
		t.Error("expected F811 for double import")
	}
}

// F821 — undefined name

func TestF821Undefined(t *testing.T) {
	ds := lint(t, "print(undefined_name)\n")
	if !hasCode(ds, "F821") {
		t.Error("expected F821 for undefined name")
	}
}

func TestF821Builtin(t *testing.T) {
	ds := lint(t, "print(len([]))\n")
	if hasCode(ds, "F821") {
		t.Error("unexpected F821 for builtins print/len")
	}
}

func TestF821StarImportSuppresses(t *testing.T) {
	ds := lint(t, "from os import *\nprint(path)\n")
	if hasCode(ds, "F821") {
		t.Error("F821 should be suppressed when star import is present")
	}
}

// F841 — unused local

func TestF841UnusedLocal(t *testing.T) {
	ds := lint(t, "def f():\n    x = 1\n    return 2\n")
	if !hasCode(ds, "F841") {
		t.Error("expected F841 for unused local x")
	}
}

func TestF841UsedLocal(t *testing.T) {
	ds := lint(t, "def f():\n    x = 1\n    return x\n")
	if hasCode(ds, "F841") {
		t.Error("unexpected F841 when local is used")
	}
}

// E711 — comparison to None

func TestE711NoneEq(t *testing.T) {
	ds := lint(t, "x == None\n")
	if !hasCode(ds, "E711") {
		t.Error("expected E711 for `x == None`")
	}
}

func TestE711NoneIs(t *testing.T) {
	ds := lint(t, "x is None\n")
	if hasCode(ds, "E711") {
		t.Error("unexpected E711 for `x is None`")
	}
}

// E712 — comparison to True/False

func TestE712TrueEq(t *testing.T) {
	ds := lint(t, "x == True\n")
	if !hasCode(ds, "E712") {
		t.Error("expected E712 for `x == True`")
	}
}

// W605 — invalid escape

func TestW605InvalidEscape(t *testing.T) {
	ds := lintFile(t, "x = \"\\p\"\n")
	if !hasCode(ds, "W605") {
		t.Error("expected W605 for invalid escape \\p")
	}
}

func TestW605ValidEscape(t *testing.T) {
	ds := lintFile(t, "x = \"\\n\"\n")
	if hasCode(ds, "W605") {
		t.Error("unexpected W605 for valid escape \\n")
	}
}

func TestW605RawString(t *testing.T) {
	ds := lintFile(t, "x = r\"\\p\"\n")
	if hasCode(ds, "W605") {
		t.Error("unexpected W605 in raw string")
	}
}

// F403 — star import

func TestF403StarImport(t *testing.T) {
	ds := lint(t, "from os import *\n")
	if !hasCode(ds, "F403") {
		t.Error("expected F403 for star import")
	}
}

func TestF403NormalImport(t *testing.T) {
	ds := lint(t, "from os import path\npath.join('a', 'b')\n")
	if hasCode(ds, "F403") {
		t.Error("unexpected F403 for normal import")
	}
}

// F631 — assert tuple

func TestF631AssertTuple(t *testing.T) {
	ds := lint(t, "assert (True, False)\n")
	if !hasCode(ds, "F631") {
		t.Error("expected F631 for assert with non-empty tuple")
	}
}

func TestF631AssertNonTuple(t *testing.T) {
	ds := lint(t, "assert True\n")
	if hasCode(ds, "F631") {
		t.Error("unexpected F631 for assert with non-tuple")
	}
}

// W291 — trailing whitespace

func TestW291TrailingWhitespace(t *testing.T) {
	ds := lintFile(t, "x = 1   \n")
	if !hasCode(ds, "W291") {
		t.Error("expected W291 for trailing whitespace")
	}
}

func TestW291NoTrailingWhitespace(t *testing.T) {
	ds := lintFile(t, "x = 1\n")
	if hasCode(ds, "W291") {
		t.Error("unexpected W291 when no trailing whitespace")
	}
}

// F901 — raise NotImplemented

func TestF901RaiseNotImplemented(t *testing.T) {
	ds := lint(t, "raise NotImplemented\n")
	if !hasCode(ds, "F901") {
		t.Error("expected F901 for raise NotImplemented")
	}
}

func TestF901RaiseNotImplementedError(t *testing.T) {
	ds := lint(t, "raise NotImplementedError\n")
	if hasCode(ds, "F901") {
		t.Error("unexpected F901 for raise NotImplementedError")
	}
}

// Diagnostic type is from diag package.
var _ diag.Diagnostic = diag.Diagnostic{}

// Verify no unexpected diagnostics when imports are used at module level.
func TestCleanCode(t *testing.T) {
	src := strings.Join([]string{
		"import os",
		"import sys",
		"",
		"path = os.path.join(sys.argv[1], 'out')",
		"print(path)",
		"",
	}, "\n")
	ds := lint(t, src)
	if len(ds) != 0 {
		t.Errorf("expected no diagnostics on clean code, got: %v", ds)
	}
}
