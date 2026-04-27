package linter

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestBaseline runs the linter over linter/testdata/baseline/*.py and
// compares the combined output to baseline.golden. The golden file is
// the contract: when a check changes its mind, refresh the golden in
// the same PR. Run `go test ./linter -update` to regenerate.
func TestBaseline(t *testing.T) {
	dir := filepath.Join("testdata", "baseline")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read baseline dir: %v", err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	sort.Strings(paths)

	var lines []string
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		// Strip the testdata/baseline/ prefix from the filename in
		// diagnostics so the golden stays portable across checkouts.
		ds, err := LintFile(filepath.Base(p), src)
		if err != nil {
			t.Fatalf("LintFile %s: %v", p, err)
		}
		for _, d := range ds {
			lines = append(lines, d.String())
		}
	}
	got := strings.Join(lines, "\n")
	if got != "" {
		got += "\n"
	}

	goldenPath := filepath.Join("testdata", "baseline.golden")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("baseline output drift\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}
