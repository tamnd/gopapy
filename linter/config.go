package linter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config controls which diagnostic codes the linter emits and which
// it skips. Empty means "all known codes enabled" so the zero Config
// matches the default Lint(mod) behaviour from v0.1.13–v0.1.15.
//
// The fields hold raw code strings rather than typed code constants so
// the config layer doesn't have to be regenerated each time a new
// check ships. Unknown codes in any list are an error at load time
// (caught once, not silently ignored).
type Config struct {
	// Select lists codes to enable. Empty means "all codes enabled";
	// a non-empty list restricts emission to those codes.
	Select []string
	// Ignore lists codes to skip. Always wins over Select.
	Ignore []string
	// PerFile maps a filepath.Match-style glob to a list of codes that
	// should be ignored for files matching the glob. Patterns are
	// matched against the filename argument as passed to LintFile and,
	// failing that, against every "/"-separated suffix of it, so a
	// glob like "tests/*" matches both "tests/x.py" and
	// "/repo/tests/x.py".
	PerFile map[string][]string
}

// Enabled reports whether `code` is allowed to fire under this Config,
// ignoring any per-file overrides. Use EnabledFor when you have a
// filename in hand.
func (c Config) Enabled(code string) bool {
	if len(c.Select) > 0 && !containsCode(c.Select, code) {
		return false
	}
	if containsCode(c.Ignore, code) {
		return false
	}
	return true
}

// EnabledFor extends Enabled with per-file-ignores. A code is
// suppressed when any matching glob lists it.
func (c Config) EnabledFor(filename, code string) bool {
	if !c.Enabled(code) {
		return false
	}
	for glob, codes := range c.PerFile {
		if !containsCode(codes, code) {
			continue
		}
		if matchPath(glob, filename) {
			return false
		}
	}
	return true
}

// AllCodes returns every diagnostic code currently emitted by the
// linter. Used by the loader to validate user-supplied lists and by
// tests that want a denylist of "everything except X".
func AllCodes() []string {
	return []string{
		CodeUnusedImport,
		CodeFStringWithoutPlaceholders,
		CodeIsWithLiteral,
		CodeRedefinitionUnused,
		CodeUnusedLocal,
	}
}

// Validate reports an error for any unknown code in Select, Ignore,
// or PerFile. Called by LoadConfig; safe to call directly when
// constructing a Config in code.
func (c Config) Validate() error {
	known := map[string]bool{}
	for _, code := range AllCodes() {
		known[code] = true
	}
	check := func(where string, codes []string) error {
		for _, code := range codes {
			if !known[code] {
				return fmt.Errorf("%s: unknown code %q", where, code)
			}
		}
		return nil
	}
	if err := check("select", c.Select); err != nil {
		return err
	}
	if err := check("ignore", c.Ignore); err != nil {
		return err
	}
	for glob, codes := range c.PerFile {
		if err := check("per-file-ignores["+glob+"]", codes); err != nil {
			return err
		}
	}
	return nil
}

// tomlRoot mirrors the namespaced layout in pyproject.toml:
//
//	[tool.gopapy.lint]
//	select = [...]
//	ignore = [...]
//	[tool.gopapy.lint.per-file-ignores]
//	"glob" = [...]
type tomlRoot struct {
	Tool struct {
		Gopapy struct {
			Lint struct {
				Select        []string            `toml:"select"`
				Ignore        []string            `toml:"ignore"`
				PerFileIgnore map[string][]string `toml:"per-file-ignores"`
			} `toml:"lint"`
		} `toml:"gopapy"`
	} `toml:"tool"`
}

// LoadConfig parses path as TOML and returns the resolved Config.
// Unknown codes produce a non-nil error so a typo in pyproject.toml
// is caught at startup rather than silently ignored.
func LoadConfig(path string) (Config, error) {
	var root tomlRoot
	if _, err := toml.DecodeFile(path, &root); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	cfg := Config{
		Select:  root.Tool.Gopapy.Lint.Select,
		Ignore:  root.Tool.Gopapy.Lint.Ignore,
		PerFile: root.Tool.Gopapy.Lint.PerFileIgnore,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// DiscoverConfig walks up from `start` looking for the nearest
// pyproject.toml. Returns the resolved Config and the path that was
// loaded. If no pyproject.toml is found, returns a zero Config and
// an empty path with no error — "no config" is not an error.
func DiscoverConfig(start string) (Config, string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return Config{}, "", err
	}
	info, err := os.Stat(abs)
	if err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	for {
		p := filepath.Join(abs, "pyproject.toml")
		if _, err := os.Stat(p); err == nil {
			cfg, err := LoadConfig(p)
			if err != nil {
				return Config{}, p, err
			}
			return cfg, p, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return Config{}, "", nil
		}
		abs = parent
	}
}

func containsCode(list []string, code string) bool {
	for _, c := range list {
		if c == code {
			return true
		}
	}
	return false
}

// matchPath returns true if glob matches name itself or any "/"-
// separated suffix of name. Lets a pattern like "tests/*" match both
// "tests/foo.py" and "/repo/tests/foo.py".
func matchPath(glob, name string) bool {
	name = filepath.ToSlash(name)
	if ok, _ := filepath.Match(glob, name); ok {
		return true
	}
	parts := strings.Split(name, "/")
	for i := 1; i < len(parts); i++ {
		sub := strings.Join(parts[i:], "/")
		if ok, _ := filepath.Match(glob, sub); ok {
			return true
		}
	}
	return false
}
