package linter

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/diag"
)

var updateGolden = flag.Bool("update", false, "update SARIF golden file")

func sarifSampleDiags() []diag.Diagnostic {
	return []diag.Diagnostic{
		{
			Filename: "src/x.py",
			Pos:      ast.Pos{Lineno: 1, ColOffset: 0},
			End:      ast.Pos{Lineno: 1, ColOffset: 8},
			Severity: diag.SeverityWarning,
			Code:     "F401",
			Msg:      "'os' imported but unused",
		},
		{
			Filename: "src/x.py",
			Pos:      ast.Pos{Lineno: 5, ColOffset: 4},
			End:      ast.Pos{Lineno: 5, ColOffset: 4},
			Severity: diag.SeverityError,
			Code:     "F821",
			Msg:      "undefined name 'foo'",
		},
		{
			Filename: "src/y.py",
			Pos:      ast.Pos{Lineno: 2, ColOffset: 0},
			Severity: diag.SeverityHint,
			Code:     "E711",
			Msg:      "comparison to None should be `if cond is None:`",
		},
	}
}

func sarifSampleTool() ToolInfo {
	return ToolInfo{
		Name:           "gopapy",
		Version:        "0.1.24",
		InformationURI: "https://github.com/tamnd/gopapy",
	}
}

func TestFormatSARIF_GoldenSchema(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSARIFLog(&buf, sarifSampleDiags(), sarifSampleTool()); err != nil {
		t.Fatalf("WriteSARIFLog: %v", err)
	}
	got := buf.Bytes()
	goldenPath := filepath.Join("testdata", "sarif", "basic.golden.json")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run `go test -update`)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("SARIF output diverged from golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatSARIF_EmptyResults(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSARIFLog(&buf, nil, sarifSampleTool()); err != nil {
		t.Fatalf("WriteSARIFLog: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	runs, ok := doc["runs"].([]any)
	if !ok || len(runs) != 1 {
		t.Fatalf("expected exactly one run, got %v", doc["runs"])
	}
	results, ok := runs[0].(map[string]any)["results"].([]any)
	if !ok {
		t.Fatalf("results must be present and an array, got %v", runs[0])
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestFormatSARIF_LevelMapping(t *testing.T) {
	cases := []struct {
		sev  diag.Severity
		want string
	}{
		{diag.SeverityError, "error"},
		{diag.SeverityWarning, "warning"},
		{diag.SeverityHint, "note"},
	}
	for _, tc := range cases {
		d := diag.Diagnostic{
			Filename: "x.py", Pos: ast.Pos{Lineno: 1, ColOffset: 0},
			Severity: tc.sev, Code: "F401", Msg: "m",
		}
		var buf bytes.Buffer
		if err := WriteSARIFLog(&buf, []diag.Diagnostic{d}, sarifSampleTool()); err != nil {
			t.Fatalf("WriteSARIFLog: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		got := doc["runs"].([]any)[0].(map[string]any)["results"].([]any)[0].(map[string]any)["level"]
		if got != tc.want {
			t.Errorf("severity %v -> level %v, want %v", tc.sev, got, tc.want)
		}
	}
}

func TestFormatSARIF_RejectedFromWriteDiagnostic(t *testing.T) {
	var buf bytes.Buffer
	err := WriteDiagnostic(&buf, sampleDiag(), FormatSARIF)
	if err == nil {
		t.Fatalf("expected error for SARIF in WriteDiagnostic")
	}
}

func TestFormatSARIF_ParseFormatAccepts(t *testing.T) {
	got, err := ParseFormat("sarif")
	if err != nil || got != FormatSARIF {
		t.Errorf("ParseFormat(sarif) = (%q, %v), want (sarif, nil)", got, err)
	}
}
