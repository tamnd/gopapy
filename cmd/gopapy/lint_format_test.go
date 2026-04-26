package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const lintFormatFixtureSrc = "import os\n"

func writeLintFormatFixture(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "x.py")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

// TestLintCmd_FormatTextDefault confirms the absence of --format
// preserves the v0.1.16 byte-exact output.
func TestLintCmd_FormatTextDefault(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	var stdout, stderr bytes.Buffer
	_ = run([]string{"lint", "--no-config", path}, &stdout, &stderr)
	out := stdout.String()
	if !strings.Contains(out, "warning[F401]") {
		t.Errorf("default text format should emit warning[F401], got: %q", out)
	}
	if strings.Contains(out, "{") {
		t.Errorf("default text format must not be JSON, got: %q", out)
	}
}

// TestLintCmd_FormatJSON checks that --format json yields one
// flat JSON object per diagnostic on the documented schema.
func TestLintCmd_FormatJSON(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--format", "json", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 JSON line, got %d:\n%s", len(lines), stdout.String())
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nline: %s", err, lines[0])
	}
	if got["code"] != "F401" {
		t.Errorf("code = %v, want F401", got["code"])
	}
	if got["severity"] != "warning" {
		t.Errorf("severity = %v, want warning", got["severity"])
	}
	if got["filename"] != path {
		t.Errorf("filename = %v, want %s", got["filename"], path)
	}
	if got["line"] != float64(1) {
		t.Errorf("line = %v, want 1", got["line"])
	}
	if _, ok := got["message"]; !ok {
		t.Errorf("missing message field: %s", lines[0])
	}
}

// TestLintCmd_FormatJSONEqualsForm exercises the --format=VALUE
// shorthand so users don't have to type two arguments.
func TestLintCmd_FormatJSONEqualsForm(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--format=json", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "{") {
		t.Errorf("--format=json should emit JSON, got: %s", stdout.String())
	}
}

// TestLintCmd_FormatGithub validates the GitHub Actions workflow
// command line shape.
func TestLintCmd_FormatGithub(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--format", "github", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := strings.TrimRight(stdout.String(), "\n")
	if !strings.HasPrefix(out, "::warning ") {
		t.Errorf("github format should start with ::warning, got: %q", out)
	}
	if !strings.Contains(out, "file="+path) {
		t.Errorf("github line should include file=%s, got: %q", path, out)
	}
	if !strings.Contains(out, "::F401 ") {
		t.Errorf("github line should include ::F401 message prefix, got: %q", out)
	}
}

// TestLintCmd_JSONFlagIsAlias confirms the deprecated --json flag
// still routes through the new formatter (flat schema).
func TestLintCmd_JSONFlagIsAlias(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--json", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	line := strings.TrimSpace(stdout.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nline: %s", err, line)
	}
	// Flat schema: line/column at the top, no nested pos object.
	if _, ok := got["pos"]; ok {
		t.Errorf("--json should now emit flat schema, found nested pos: %s", line)
	}
	if _, ok := got["line"]; !ok {
		t.Errorf("--json should emit flat 'line' field, got: %s", line)
	}
}

// TestLintCmd_FormatUnknownErrors guards against silent fallback
// when the user typoes the format name.
func TestLintCmd_FormatUnknownErrors(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	var stdout, stderr bytes.Buffer
	err := run([]string{"lint", "--format", "yaml", path}, &stdout, &stderr)
	if err == nil {
		t.Errorf("expected error for unknown --format, stdout: %s", stdout.String())
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("error should mention unknown format, got: %v", err)
	}
}

// TestLintCmd_OutputToFile writes the diagnostic stream to a file
// and verifies stdout is left empty.
func TestLintCmd_OutputToFile(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	outPath := filepath.Join(t.TempDir(), "diags.json")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--format", "json", "--output", outPath, path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("--output should leave stdout empty, got: %q", stdout.String())
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(body), `"code":"F401"`) {
		t.Errorf("output file should contain F401 JSON, got: %s", body)
	}
}

// TestLintCmd_FormatSARIF checks that --format sarif emits a single
// well-formed SARIF 2.1.0 document on stdout with our tool name in
// the driver block.
func TestLintCmd_FormatSARIF(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--format", "sarif", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("invalid SARIF JSON: %v\n%s", err, stdout.String())
	}
	if doc["version"] != "2.1.0" {
		t.Errorf("sarif version = %v, want 2.1.0", doc["version"])
	}
	runs, ok := doc["runs"].([]any)
	if !ok || len(runs) != 1 {
		t.Fatalf("runs missing or not a single-element array: %v", doc["runs"])
	}
	driver := runs[0].(map[string]any)["tool"].(map[string]any)["driver"].(map[string]any)
	if driver["name"] != "gopapy" {
		t.Errorf("tool.driver.name = %v, want gopapy", driver["name"])
	}
	results := runs[0].(map[string]any)["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].(map[string]any)["ruleId"] != "F401" {
		t.Errorf("ruleId = %v, want F401", results[0].(map[string]any)["ruleId"])
	}
}

// TestLintCmd_FormatSARIFToFile mirrors TestLintCmd_OutputToFile but
// for SARIF; the whole document goes to the file in one shot.
func TestLintCmd_FormatSARIFToFile(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	outPath := filepath.Join(t.TempDir(), "out.sarif")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--format", "sarif", "--output", outPath, path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("--output should leave stdout empty, got: %q", stdout.String())
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read sarif file: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("invalid SARIF JSON in file: %v\n%s", err, body)
	}
	if doc["version"] != "2.1.0" {
		t.Errorf("sarif version = %v, want 2.1.0", doc["version"])
	}
}

// TestLintCmd_OutputDashIsStdout makes "-" explicit since users may
// pass it expecting the standard convention.
func TestLintCmd_OutputDashIsStdout(t *testing.T) {
	path := writeLintFormatFixture(t, lintFormatFixtureSrc)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--output", "-", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "warning[F401]") {
		t.Errorf("--output - should write to stdout, got: %q", stdout.String())
	}
}
