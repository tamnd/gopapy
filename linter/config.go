package linter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config controls which diagnostic codes the linter emits and which
// it skips. Empty means "all known codes enabled".
type Config struct {
	Select  []string
	Ignore  []string
	PerFile map[string][]string
}

// Enabled reports whether code is allowed under this Config, ignoring
// per-file overrides.
func (c Config) Enabled(code string) bool {
	if len(c.Select) > 0 && !containsCode(c.Select, code) {
		return false
	}
	if containsCode(c.Ignore, code) {
		return false
	}
	return true
}

// EnabledFor extends Enabled with per-file ignores.
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

// AllCodes lists every diagnostic code the linter currently emits.
func AllCodes() []string {
	return []string{
		CodeUnusedImport,
		CodeStarImport,
		CodePercentFormatMismatch,
		CodeFStringWithoutPlaceholders,
		CodeAssertTuple,
		CodeIsWithLiteral,
		CodeRedefinitionUnused,
		CodeUndefinedName,
		CodeUnusedLocal,
		CodeRaiseNotImplemented,
		CodeComparisonToNone,
		CodeComparisonToBool,
		CodeTrailingWhitespace,
		CodeInvalidEscape,
	}
}

// Validate reports an error for any unknown code.
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

// LoadConfig parses path as TOML.
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

// DiscoverConfig walks up from start looking for the nearest
// pyproject.toml.
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
// separated suffix of name.
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
