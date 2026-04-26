package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLintCmd_StdinText reads a buffer from stdin and produces the
// usual text-formatted diagnostics on stdout.
func TestLintCmd_StdinText(t *testing.T) {
	src := bytes.NewReader([]byte("import os\n"))
	var stdout, stderr bytes.Buffer
	if err := runWithStdin([]string{"lint", "--no-config", "-"}, src, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "warning[F401]") {
		t.Errorf("stdin lint should emit F401, got: %q", out)
	}
	if !strings.Contains(out, "<stdin>") {
		t.Errorf("default stdin filename should be <stdin>, got: %q", out)
	}
}

// TestLintCmd_StdinJSON exercises the JSON encoder with stdin input.
func TestLintCmd_StdinJSON(t *testing.T) {
	src := bytes.NewReader([]byte("import os\n"))
	var stdout, stderr bytes.Buffer
	if err := runWithStdin([]string{"lint", "--no-config", "--format", "json", "-"}, src, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	line := strings.TrimSpace(stdout.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nline: %s", err, line)
	}
	if got["filename"] != "<stdin>" {
		t.Errorf("default JSON filename = %v, want <stdin>", got["filename"])
	}
	if got["code"] != "F401" {
		t.Errorf("code = %v, want F401", got["code"])
	}
}

// TestLintCmd_StdinFilename confirms --stdin-filename controls the
// filename in the output (so editors can jump to source).
func TestLintCmd_StdinFilename(t *testing.T) {
	src := bytes.NewReader([]byte("import os\n"))
	var stdout, stderr bytes.Buffer
	err := runWithStdin([]string{
		"lint", "--no-config", "--stdin-filename", "src/x.py", "-",
	}, src, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "src/x.py:") {
		t.Errorf("expected diagnostic prefixed with src/x.py, got: %q", stdout.String())
	}
}

// TestLintCmd_StdinFilenamePerFileIgnore makes sure per-file ignores
// match against the user-supplied stdin filename, not against
// "<stdin>".
func TestLintCmd_StdinFilenamePerFileIgnore(t *testing.T) {
	dir := t.TempDir()
	cfg := `[tool.gopapy.lint.per-file-ignores]
"tests/*" = ["F401"]
`
	cfgPath := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	src := bytes.NewReader([]byte("import os\n"))
	var stdout, stderr bytes.Buffer
	err := runWithStdin([]string{
		"lint", "--config", cfgPath,
		"--stdin-filename", "tests/x.py", "-",
	}, src, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "F401") {
		t.Errorf("per-file ignore should suppress F401 for tests/x.py, got: %q", stdout.String())
	}
}

// TestLintCmd_StdinDiscoveryAnchor confirms that when --stdin-filename
// points into a project tree, that path is used as the discovery
// anchor and the project's pyproject.toml is loaded.
func TestLintCmd_StdinDiscoveryAnchor(t *testing.T) {
	dir := t.TempDir()
	cfg := `[tool.gopapy.lint]
ignore = ["F401"]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	bufferPath := filepath.Join(dir, "src", "x.py") // does not need to exist
	src := bytes.NewReader([]byte("import os\n"))
	var stdout, stderr bytes.Buffer
	if err := runWithStdin([]string{
		"lint", "--stdin-filename", bufferPath, "-",
	}, src, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "F401") {
		t.Errorf("discovered config should suppress F401, got: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "loaded config from") {
		t.Errorf("expected stderr to announce config load, got: %q", stderr.String())
	}
}

// TestLintCmd_StdinFix writes the rewritten source to stdout and
// the remaining diagnostics to stderr.
func TestLintCmd_StdinFix(t *testing.T) {
	src := bytes.NewReader([]byte("import os\nimport sys\nprint(sys)\n"))
	var stdout, stderr bytes.Buffer
	if err := runWithStdin([]string{
		"lint", "--no-config", "--fix", "-",
	}, src, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	if got := stdout.String(); got != "import sys\nprint(sys)\n" {
		t.Errorf("stdout should hold rewritten source, got: %q", got)
	}
	// After the fix nothing remains for the linter to flag, but the
	// trailing summary still has to land on stderr.
	if !strings.Contains(stderr.String(), "fixes applied") {
		t.Errorf("stderr should report fix summary, got: %q", stderr.String())
	}
}

// TestLintCmd_StdinFixNoChange exercises the round-trip when nothing
// is fixable: stdout still receives the source verbatim so the
// editor can reload unconditionally.
func TestLintCmd_StdinFixNoChange(t *testing.T) {
	body := "import os\nprint(os)\n"
	src := bytes.NewReader([]byte(body))
	var stdout, stderr bytes.Buffer
	if err := runWithStdin([]string{
		"lint", "--no-config", "--fix", "-",
	}, src, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.String() != body {
		t.Errorf("no-fix path should echo source verbatim, got: %q", stdout.String())
	}
}

// TestLintCmd_StdinEmpty must not panic on an empty buffer.
func TestLintCmd_StdinEmpty(t *testing.T) {
	src := bytes.NewReader(nil)
	var stdout, stderr bytes.Buffer
	if err := runWithStdin([]string{"lint", "--no-config", "-"}, src, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("empty stdin should produce no diagnostics, got: %q", stdout.String())
	}
}
