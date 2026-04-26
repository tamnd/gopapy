package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const lintFixtureSrc = "import os\nx = f\"static\"\nif x is 1: pass\n"

func writeLintFixture(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

// TestLintCmd_ConfigDiscoveryIgnoresF541 places a pyproject.toml next
// to the fixture and confirms the linter respects its ignore list.
func TestLintCmd_ConfigDiscoveryIgnoresF541(t *testing.T) {
	dir := t.TempDir()
	cfg := `[tool.gopapy.lint]
ignore = ["F541", "F632"]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	path := writeLintFixture(t, dir, "x.py", lintFixtureSrc)

	var stdout, stderr bytes.Buffer
	_ = run([]string{"lint", path}, &stdout, &stderr)
	out := stdout.String()
	if strings.Contains(out, "warning[F541]") {
		t.Errorf("expected F541 suppressed by config, got:\n%s", out)
	}
	if strings.Contains(out, "warning[F632]") {
		t.Errorf("expected F632 suppressed by config, got:\n%s", out)
	}
	if !strings.Contains(out, "warning[F401]") {
		t.Errorf("F401 should still fire, got:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "loaded config from") {
		t.Errorf("stderr should announce config load, got: %s", stderr.String())
	}
}

// TestLintCmd_NoConfigFlag bypasses discovery so the config in the
// fixture directory is ignored.
func TestLintCmd_NoConfigFlag(t *testing.T) {
	dir := t.TempDir()
	cfg := `[tool.gopapy.lint]
ignore = ["F541", "F632", "F401"]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	path := writeLintFixture(t, dir, "x.py", lintFixtureSrc)

	var stdout, stderr bytes.Buffer
	_ = run([]string{"lint", "--no-config", path}, &stdout, &stderr)
	for _, code := range []string{"F401", "F541", "F632"} {
		if !strings.Contains(stdout.String(), "warning["+code+"]") {
			t.Errorf("--no-config should restore %s, output:\n%s", code, stdout.String())
		}
	}
	if strings.Contains(stderr.String(), "loaded config from") {
		t.Errorf("stderr must not announce a config under --no-config: %s", stderr.String())
	}
}

// TestLintCmd_ExplicitConfigFlag points at a config file in a
// different directory.
func TestLintCmd_ExplicitConfigFlag(t *testing.T) {
	srcDir := t.TempDir()
	cfgDir := t.TempDir()
	cfg := `[tool.gopapy.lint]
ignore = ["F541"]
`
	cfgPath := filepath.Join(cfgDir, "custom.toml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	path := writeLintFixture(t, srcDir, "x.py", lintFixtureSrc)

	var stdout, stderr bytes.Buffer
	_ = run([]string{"lint", "--config", cfgPath, path}, &stdout, &stderr)
	if strings.Contains(stdout.String(), "warning[F541]") {
		t.Errorf("--config should suppress F541, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), cfgPath) {
		t.Errorf("stderr should report loaded config path, got: %s", stderr.String())
	}
}

// TestLintCmd_FixHonoursPerFileIgnore exercises the F401 fix being
// gated by a per-file-ignore.
func TestLintCmd_FixHonoursPerFileIgnore(t *testing.T) {
	dir := t.TempDir()
	cfg := `[tool.gopapy.lint.per-file-ignores]
"tests/*" = ["F401"]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	testsDir := filepath.Join(dir, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	srcPath := writeLintFixture(t, testsDir, "x.py", "import os\n")

	var stdout, stderr bytes.Buffer
	_ = run([]string{"lint", "--fix", srcPath}, &stdout, &stderr)
	got, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read after fix: %v", err)
	}
	if string(got) != "import os\n" {
		t.Errorf("expected file untouched (per-file-ignore covers F401), got:\n%s", got)
	}
}

// TestLintCmd_FixWithDefaultConfig confirms F401 fix still works when
// no config restricts it.
func TestLintCmd_FixWithDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	srcPath := writeLintFixture(t, dir, "x.py", "import os\nimport sys\nprint(sys)\n")

	var stdout, stderr bytes.Buffer
	_ = run([]string{"lint", "--fix", "--no-config", srcPath}, &stdout, &stderr)
	got, _ := os.ReadFile(srcPath)
	if string(got) != "import sys\nprint(sys)\n" {
		t.Errorf("expected unused import removed, got:\n%s", got)
	}
}
