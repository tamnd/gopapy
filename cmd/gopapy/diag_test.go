package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture: nested function with conflicting global / nonlocal — the
// only diagnostic the v0.1.7 symbols pass currently emits.
const conflictSrc = `def outer():
    x = 0
    def inner():
        global x
        nonlocal x
`

func writeFixture(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "conflict.py")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func TestDiagCmd_HumanOutput(t *testing.T) {
	path := writeFixture(t, conflictSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"diag", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, path) {
		t.Errorf("output missing filename %q:\n%s", path, out)
	}
	if !strings.Contains(out, "warning[S001]") {
		t.Errorf("output missing 'warning[S001]':\n%s", out)
	}
	if !strings.Contains(out, "global") || !strings.Contains(out, "nonlocal") {
		t.Errorf("output missing message text:\n%s", out)
	}
}

func TestDiagCmd_JSONOutput(t *testing.T) {
	path := writeFixture(t, conflictSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"diag", "--json", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("no JSONL output")
	}
	for i, line := range lines {
		var d map[string]any
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			t.Fatalf("line %d not JSON: %v\nline: %q", i, err, line)
		}
		if d["severity"] != "warning" {
			t.Errorf("line %d severity = %v", i, d["severity"])
		}
		if d["code"] != "S001" {
			t.Errorf("line %d code = %v", i, d["code"])
		}
		if d["filename"] != path {
			t.Errorf("line %d filename = %v want %v", i, d["filename"], path)
		}
	}
}

func TestDiagCmd_NoFindings(t *testing.T) {
	path := writeFixture(t, "x = 1\n")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"diag", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no diagnostics, got: %s", stdout.String())
	}
}

func TestDiagCmd_Directory(t *testing.T) {
	dir := t.TempDir()
	conflictPath := filepath.Join(dir, "conflict.py")
	cleanPath := filepath.Join(dir, "clean.py")
	if err := os.WriteFile(conflictPath, []byte(conflictSrc), 0o644); err != nil {
		t.Fatalf("write conflict fixture: %v", err)
	}
	if err := os.WriteFile(cleanPath, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatalf("write clean fixture: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"diag", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), conflictPath) {
		t.Errorf("diag output missing %s:\n%s", conflictPath, stdout.String())
	}
	if !strings.Contains(stderr.String(), "2 files") {
		t.Errorf("summary missing '2 files' from stderr: %s", stderr.String())
	}
}

func TestDiagCmd_MissingPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"diag"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected error for missing PATH")
	}
}

func TestDiagCmd_JSONFlagFirst(t *testing.T) {
	path := writeFixture(t, conflictSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"diag", "--json", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !json.Valid(bytes.TrimSpace(stdout.Bytes())) && !strings.Contains(stdout.String(), "{") {
		t.Errorf("expected JSON output, got: %s", stdout.String())
	}
}
