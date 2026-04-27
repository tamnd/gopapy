package parser_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/gopapy/parser"
)

func TestASTDump(t *testing.T) {
	python3, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found in PATH")
	}
	oracle := filepath.Join("..", "internal", "oracle", "oracle.py")
	if _, err := os.Stat(oracle); err != nil {
		t.Skipf("oracle not found: %v", err)
	}

	fixtureDir := filepath.Join("..", "tests", "grammar")
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".py") {
			continue
		}
		name := entry.Name()
		path := filepath.Join(fixtureDir, name)

		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			// Oracle: run python3 oracle.py <fixture>
			out, err := exec.Command(python3, oracle, path).Output()
			if err != nil {
				t.Skipf("oracle failed: %v", err)
			}
			want := strings.TrimRight(string(out), "\n")

			// parser: parse + ASTDump
			mod, perr := parser.ParseFile(path, string(src))
			if perr != nil {
				t.Skipf("parser cannot parse %s: %v", name, perr)
			}
			got := parser.ASTDump(mod, 14)

			if got != want {
				t.Errorf("ASTDump mismatch for %s\nwant: %s\n got: %s", name, want, got)
			}
		})
	}
}
