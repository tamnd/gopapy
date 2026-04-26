package diag

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tamnd/gopapy/v1/ast"
)

func TestSeverityString(t *testing.T) {
	cases := []struct {
		s    Severity
		want string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{SeverityHint, "hint"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestSeverityOrdering(t *testing.T) {
	if !(SeverityError < SeverityWarning) {
		t.Fatalf("SeverityError must be < SeverityWarning so error sorts first")
	}
	if !(SeverityWarning < SeverityHint) {
		t.Fatalf("SeverityWarning must be < SeverityHint so warnings sort before hints")
	}
}

func TestDiagnosticString(t *testing.T) {
	d := Diagnostic{
		Filename: "foo.py",
		Pos:      ast.Pos{Lineno: 12, ColOffset: 4, EndLineno: 12, EndColOffset: 9},
		End:      ast.Pos{Lineno: 12, ColOffset: 4, EndLineno: 12, EndColOffset: 9},
		Severity: SeverityWarning,
		Code:     "S001",
		Msg:      "name x is both nonlocal and global",
	}
	want := "foo.py:12:4: warning[S001]: name x is both nonlocal and global"
	if got := d.String(); got != want {
		t.Errorf("\n got: %q\nwant: %q", got, want)
	}
}

func TestDiagnosticString_NoFilename(t *testing.T) {
	d := Diagnostic{
		Pos:      ast.Pos{Lineno: 3, ColOffset: 0},
		Severity: SeverityError,
		Code:     "E001",
		Msg:      "boom",
	}
	want := "3:0: error[E001]: boom"
	if got := d.String(); got != want {
		t.Errorf("\n got: %q\nwant: %q", got, want)
	}
}

func TestDiagnosticString_NoCode(t *testing.T) {
	d := Diagnostic{
		Filename: "a.py",
		Pos:      ast.Pos{Lineno: 1, ColOffset: 0},
		Severity: SeverityHint,
		Msg:      "consider X",
	}
	want := "a.py:1:0: hint: consider X"
	if got := d.String(); got != want {
		t.Errorf("\n got: %q\nwant: %q", got, want)
	}
}

func TestDiagnosticJSON(t *testing.T) {
	d := Diagnostic{
		Filename: "foo.py",
		Pos:      ast.Pos{Lineno: 1, ColOffset: 0, EndLineno: 1, EndColOffset: 3},
		End:      ast.Pos{Lineno: 1, ColOffset: 0, EndLineno: 1, EndColOffset: 3},
		Severity: SeverityWarning,
		Code:     "S002",
		Msg:      "no enclosing binding",
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["filename"] != "foo.py" {
		t.Errorf("filename: %v", got["filename"])
	}
	if got["severity"] != "warning" {
		t.Errorf("severity: %v", got["severity"])
	}
	if got["code"] != "S002" {
		t.Errorf("code: %v", got["code"])
	}
	if got["msg"] != "no enclosing binding" {
		t.Errorf("msg: %v", got["msg"])
	}
	if !strings.Contains(string(b), `"pos":`) || !strings.Contains(string(b), `"end":`) {
		t.Errorf("missing pos/end keys: %s", b)
	}
}

func TestDiagnosticJSON_OmitsEmpty(t *testing.T) {
	d := Diagnostic{
		Pos:      ast.Pos{Lineno: 1, ColOffset: 0},
		Severity: SeverityError,
		Msg:      "boom",
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), `"filename"`) {
		t.Errorf("expected filename omitted, got %s", b)
	}
	if strings.Contains(string(b), `"code"`) {
		t.Errorf("expected code omitted, got %s", b)
	}
}
