package linter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigEnabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		code string
		want bool
	}{
		{"empty-config-enables-everything", Config{}, "F401", true},
		{"select-allows-listed", Config{Select: []string{"F401"}}, "F401", true},
		{"select-rejects-unlisted", Config{Select: []string{"F401"}}, "F811", false},
		{"ignore-rejects-listed", Config{Ignore: []string{"F541"}}, "F541", false},
		{"ignore-overrides-select", Config{Select: []string{"F401"}, Ignore: []string{"F401"}}, "F401", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.Enabled(tc.code)
			if got != tc.want {
				t.Errorf("Enabled(%q) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

func TestConfigEnabledFor(t *testing.T) {
	cfg := Config{
		Ignore: []string{"F541"},
		PerFile: map[string][]string{
			"tests/*":      {"F401", "F841"},
			"_vendor/*.py": {"F811"},
		},
	}
	cases := []struct {
		name     string
		filename string
		code     string
		want     bool
	}{
		{"global-ignore-still-applies", "src/x.py", "F541", false},
		{"per-file-glob-matches", "tests/x.py", "F401", false},
		{"per-file-glob-not-applicable-code", "tests/x.py", "F811", true},
		{"per-file-glob-no-match", "src/x.py", "F401", true},
		{"per-file-vendor-match", "_vendor/foo.py", "F811", false},
		{"abs-path-suffix-match", "/repo/tests/x.py", "F401", false},
		{"empty-filename-no-per-file-applied", "", "F401", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cfg.EnabledFor(tc.filename, tc.code)
			if got != tc.want {
				t.Errorf("EnabledFor(%q, %q) = %v, want %v", tc.filename, tc.code, got, tc.want)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"empty-ok", Config{}, ""},
		{"all-known-ok", Config{Select: AllCodes()}, ""},
		{"unknown-in-select", Config{Select: []string{"X999"}}, "select: unknown code"},
		{"unknown-in-ignore", Config{Ignore: []string{"X999"}}, "ignore: unknown code"},
		{"unknown-in-per-file", Config{PerFile: map[string][]string{"*": {"X999"}}}, "per-file-ignores"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("got nil, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("got %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	contents := `
[tool.poetry]
name = "demo"

[tool.gopapy.lint]
select = ["F401", "F811", "F841"]
ignore = ["F541"]

[tool.gopapy.lint.per-file-ignores]
"tests/*" = ["F401", "F841"]
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Select) != 3 || cfg.Select[0] != "F401" {
		t.Errorf("Select = %v, want [F401 F811 F841]", cfg.Select)
	}
	if len(cfg.Ignore) != 1 || cfg.Ignore[0] != "F541" {
		t.Errorf("Ignore = %v, want [F541]", cfg.Ignore)
	}
	if codes := cfg.PerFile["tests/*"]; len(codes) != 2 {
		t.Errorf("PerFile[tests/*] = %v, want 2 codes", codes)
	}
	// Co-existence with non-gopapy sections must not error.
	if !cfg.Enabled("F401") {
		t.Errorf("F401 should be enabled under select=[F401,...]")
	}
	if cfg.Enabled("F541") {
		t.Errorf("F541 should be ignored")
	}
	if cfg.EnabledFor("tests/foo.py", "F401") {
		t.Errorf("F401 should be per-file-ignored under tests/*")
	}
}

func TestLoadConfigUnknownCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	contents := `
[tool.gopapy.lint]
select = ["F401", "FXXX"]
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatalf("LoadConfig: expected error for unknown code")
	}
	if !strings.Contains(err.Error(), "FXXX") {
		t.Errorf("error = %v, want it to mention FXXX", err)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err == nil {
		t.Errorf("LoadConfig: expected error for missing file")
	}
}

func TestLoadConfigMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte("not = valid = toml"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Errorf("LoadConfig: expected error for malformed TOML")
	}
}

func TestDiscoverConfigWalkUp(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "src", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	contents := `[tool.gopapy.lint]
ignore = ["F541"]
`
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, found, err := DiscoverConfig(nested)
	if err != nil {
		t.Fatalf("DiscoverConfig: %v", err)
	}
	if found == "" {
		t.Fatalf("expected to find pyproject.toml")
	}
	if cfg.Ignore[0] != "F541" {
		t.Errorf("Ignore[0] = %q, want F541", cfg.Ignore[0])
	}
}

func TestDiscoverConfigNotFound(t *testing.T) {
	root := t.TempDir()
	cfg, found, err := DiscoverConfig(root)
	if err != nil {
		t.Errorf("DiscoverConfig: %v", err)
	}
	if found != "" {
		t.Errorf("found = %q, want empty", found)
	}
	if len(cfg.Select) != 0 || len(cfg.Ignore) != 0 {
		t.Errorf("got non-empty config %+v, want zero", cfg)
	}
}

func TestDiscoverConfigStartIsFile(t *testing.T) {
	root := t.TempDir()
	contents := `[tool.gopapy.lint]
ignore = ["F541"]
`
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srcFile := filepath.Join(root, "x.py")
	if err := os.WriteFile(srcFile, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, found, err := DiscoverConfig(srcFile)
	if err != nil {
		t.Fatalf("DiscoverConfig: %v", err)
	}
	if found == "" {
		t.Errorf("expected to find pyproject.toml from a file start")
	}
	if cfg.Ignore[0] != "F541" {
		t.Errorf("Ignore[0] = %q, want F541", cfg.Ignore[0])
	}
}

func TestLintWithConfigFiltersCodes(t *testing.T) {
	src := []byte("import os\nx = f\"static\"\nif x is 1: pass\n")
	cfg := Config{Ignore: []string{"F541", "F632"}}
	diags, err := LintFileWithConfig("a.py", src, cfg)
	if err != nil {
		t.Fatalf("LintFileWithConfig: %v", err)
	}
	for _, d := range diags {
		if d.Code == "F541" || d.Code == "F632" {
			t.Errorf("ignored code %q still emitted: %s", d.Code, d.String())
		}
	}
}

func TestLintFileWithConfigPerFile(t *testing.T) {
	src := []byte("import os\n")
	cfg := Config{PerFile: map[string][]string{"tests/*": {"F401"}}}
	diags, _ := LintFileWithConfig("tests/x.py", src, cfg)
	for _, d := range diags {
		if d.Code == "F401" {
			t.Errorf("per-file-ignored F401 still emitted at %s", d.String())
		}
	}
	// Same code MUST still fire for a file outside the glob.
	diags, _ = LintFileWithConfig("src/x.py", src, cfg)
	if len(diags) == 0 || diags[0].Code != "F401" {
		t.Errorf("expected F401 for src/x.py, got %v", diags)
	}
}
