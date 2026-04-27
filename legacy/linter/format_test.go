package linter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/legacy/diag"
)

func sampleDiag() diag.Diagnostic {
	return diag.Diagnostic{
		Filename: "src/x.py",
		Pos:      ast.Pos{Lineno: 1, ColOffset: 0},
		End:      ast.Pos{Lineno: 1, ColOffset: 8},
		Severity: diag.SeverityWarning,
		Code:     "F401",
		Msg:      "'os' imported but unused",
	}
}

func TestParseFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"text", FormatText, false},
		{"json", FormatJSON, false},
		{"github", FormatGithub, false},
		{"", "", true},
		{"yaml", "", true},
	}
	for _, tc := range cases {
		got, err := ParseFormat(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseFormat(%q) want error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseFormat(%q) = (%q, %v), want (%q, nil)", tc.in, got, err, tc.want)
		}
	}
}

func TestWriteDiagnostic_Text(t *testing.T) {
	var buf bytes.Buffer
	d := sampleDiag()
	if err := WriteDiagnostic(&buf, d, FormatText); err != nil {
		t.Fatalf("WriteDiagnostic: %v", err)
	}
	got := buf.String()
	want := d.String() + "\n"
	if got != want {
		t.Errorf("text format mismatch\n  got:  %q\n  want: %q", got, want)
	}
}

func TestWriteDiagnostic_TextEmptyFormatActsAsText(t *testing.T) {
	// Empty Format must not silently swap to JSON; it should preserve
	// the human-readable default. Belt-and-braces: callers that forget
	// to set the format still get sensible output.
	var buf bytes.Buffer
	d := sampleDiag()
	if err := WriteDiagnostic(&buf, d, ""); err != nil {
		t.Fatalf("WriteDiagnostic: %v", err)
	}
	if !strings.Contains(buf.String(), "warning[F401]") {
		t.Errorf("empty format should render as text, got %q", buf.String())
	}
}

func TestWriteDiagnostic_JSON(t *testing.T) {
	var buf bytes.Buffer
	d := sampleDiag()
	if err := WriteDiagnostic(&buf, d, FormatJSON); err != nil {
		t.Fatalf("WriteDiagnostic: %v", err)
	}
	line := strings.TrimRight(buf.String(), "\n")
	if strings.Contains(line, "\n") {
		t.Errorf("JSON line should not contain interior newline: %q", line)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nline: %s", err, line)
	}
	want := map[string]any{
		"filename":   "src/x.py",
		"line":       float64(1),
		"column":     float64(0),
		"end_line":   float64(1),
		"end_column": float64(8),
		"severity":   "warning",
		"code":       "F401",
		"message":    "'os' imported but unused",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("field %q = %v (%T), want %v", k, got[k], got[k], v)
		}
	}
}

func TestWriteDiagnostic_JSONOmitsZeroEnd(t *testing.T) {
	d := sampleDiag()
	d.End = ast.Pos{}
	var buf bytes.Buffer
	if err := WriteDiagnostic(&buf, d, FormatJSON); err != nil {
		t.Fatalf("WriteDiagnostic: %v", err)
	}
	line := strings.TrimRight(buf.String(), "\n")
	if strings.Contains(line, "end_line") || strings.Contains(line, "end_column") {
		t.Errorf("zero End should be omitted from JSON, got: %s", line)
	}
}

func TestWriteDiagnostic_Github(t *testing.T) {
	var buf bytes.Buffer
	d := sampleDiag()
	if err := WriteDiagnostic(&buf, d, FormatGithub); err != nil {
		t.Fatalf("WriteDiagnostic: %v", err)
	}
	want := "::warning file=src/x.py,line=1,col=1::F401 'os' imported but unused\n"
	if buf.String() != want {
		t.Errorf("github format mismatch\n  got:  %q\n  want: %q", buf.String(), want)
	}
}

func TestWriteDiagnostic_GithubSeverityMap(t *testing.T) {
	cases := []struct {
		sev  diag.Severity
		want string
	}{
		{diag.SeverityError, "::error"},
		{diag.SeverityWarning, "::warning"},
		{diag.SeverityHint, "::notice"},
	}
	for _, tc := range cases {
		d := sampleDiag()
		d.Severity = tc.sev
		var buf bytes.Buffer
		if err := WriteDiagnostic(&buf, d, FormatGithub); err != nil {
			t.Fatalf("WriteDiagnostic: %v", err)
		}
		if !strings.HasPrefix(buf.String(), tc.want) {
			t.Errorf("severity %v should map to %q prefix, got %q", tc.sev, tc.want, buf.String())
		}
	}
}

func TestWriteDiagnostic_GithubEscapesMessage(t *testing.T) {
	d := sampleDiag()
	d.Msg = "broken: line one\nline two::end"
	var buf bytes.Buffer
	if err := WriteDiagnostic(&buf, d, FormatGithub); err != nil {
		t.Fatalf("WriteDiagnostic: %v", err)
	}
	out := buf.String()
	// Message must collapse onto one line and not break GH's `::`
	// delimiter; otherwise the annotation parser truncates.
	if strings.Count(out, "\n") != 1 {
		t.Errorf("github line should end in exactly one newline, got %q", out)
	}
	// Two occurrences of `::` are expected: the leading `::warning`
	// directive and the `::` separator before the message body. A
	// third would mean a literal `::` from the message survived and
	// would derail GH's parser.
	if strings.Count(out, "::") != 2 {
		t.Errorf("expected exactly two `::` (level + delimiter), got %q", out)
	}
}

func TestWriteDiagnostic_UnknownFormatErrors(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteDiagnostic(&buf, sampleDiag(), Format("yaml")); err == nil {
		t.Errorf("expected error for unknown format")
	}
}
