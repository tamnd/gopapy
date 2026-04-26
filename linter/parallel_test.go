package linter

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLintFiles_DeterministicOutput verifies that LintFiles produces
// byte-identical results for the same paths regardless of Jobs. The
// determinism contract is "given (paths, cfg), output is identical
// across worker counts" — without it parallel mode can't replace the
// serial path.
func TestLintFiles_DeterministicOutput(t *testing.T) {
	dir := t.TempDir()
	const nFiles = 50
	var paths []string
	for i := 0; i < nFiles; i++ {
		// Half the files have an unused import (F401), the other half
		// are clean. Mixing makes order-of-emission visible if we get
		// the result-collection logic wrong.
		var src string
		if i%2 == 0 {
			src = "import os\n"
		} else {
			src = "import os\nprint(os.getcwd())\n"
		}
		p := filepath.Join(dir, "f"+strconv.Itoa(i)+".py")
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
		paths = append(paths, p)
	}
	SortPaths(paths) // input is already lexical from the loop, but make it explicit

	serial := LintFiles(paths, Config{}, LintOptions{Jobs: 1})
	parallel := LintFiles(paths, Config{}, LintOptions{Jobs: 8})

	if len(serial) != len(parallel) {
		t.Fatalf("len mismatch: serial=%d parallel=%d", len(serial), len(parallel))
	}
	for i := range serial {
		if serial[i].Path != parallel[i].Path {
			t.Errorf("path[%d]: serial=%q parallel=%q", i,
				serial[i].Path, parallel[i].Path)
		}
		if len(serial[i].Diagnostics) != len(parallel[i].Diagnostics) {
			t.Errorf("file %s: diag count differs (serial=%d parallel=%d)",
				serial[i].Path, len(serial[i].Diagnostics),
				len(parallel[i].Diagnostics))
			continue
		}
		for j := range serial[i].Diagnostics {
			if serial[i].Diagnostics[j].Code != parallel[i].Diagnostics[j].Code {
				t.Errorf("file %s diag[%d]: code differs (serial=%s parallel=%s)",
					serial[i].Path, j,
					serial[i].Diagnostics[j].Code,
					parallel[i].Diagnostics[j].Code)
			}
		}
	}
}

// TestLintFiles_ErrorIsPerFile verifies that a parse failure on one
// file doesn't stop the pool from working on others. The result
// slot for the broken file carries Err; siblings produce diagnostics
// as normal.
func TestLintFiles_ErrorIsPerFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.py")
	if err := os.WriteFile(good, []byte("import os\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.py") // never created
	paths := []string{good, missing}

	results := LintFiles(paths, Config{}, LintOptions{Jobs: 2})
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	// Results are in input order (path order from caller).
	if results[0].Err != nil {
		t.Errorf("good.py result has Err = %v, want nil", results[0].Err)
	}
	if len(results[0].Diagnostics) != 1 || results[0].Diagnostics[0].Code != "F401" {
		t.Errorf("good.py: want 1 F401 diag, got %v", results[0].Diagnostics)
	}
	if results[1].Err == nil {
		t.Errorf("missing.py result has nil Err, want non-nil")
	}
}

// TestLintFiles_EmptyPaths returns nil rather than panicking on an
// empty slice — the CLI may be invoked on a directory that contains
// no .py files.
func TestLintFiles_EmptyPaths(t *testing.T) {
	if got := LintFiles(nil, Config{}, LintOptions{}); got != nil {
		t.Errorf("LintFiles(nil) = %v, want nil", got)
	}
}

// TestSortPaths checks the small helper. We could call sort.Strings
// directly; SortPaths exists so callers can stay inside the linter
// package.
func TestSortPaths(t *testing.T) {
	got := SortPaths([]string{"b", "a", "c"})
	want := []string{"a", "b", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("SortPaths = %v, want %v", got, want)
	}
}
