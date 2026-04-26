package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// makeMultiFileFixture creates n .py files under a temp directory.
// Even-indexed files trigger F401, odd are clean — same shape as the
// linter-package test, repeated here to keep CLI tests self-contained.
func makeMultiFileFixture(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
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
	}
	return dir
}

// TestLintCmd_JobsParallelMatchesSerial confirms `--jobs 4` produces
// byte-identical stdout to `--jobs 1` over the same fixture set.
// Determinism is the contract: any drift here means an editor or CI
// run would see flaky diagnostic ordering.
func TestLintCmd_JobsParallelMatchesSerial(t *testing.T) {
	dir := makeMultiFileFixture(t, 30)
	var serialOut, serialErr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--jobs", "1", dir}, &serialOut, &serialErr); err != nil {
		t.Fatalf("serial: %v\nstderr: %s", err, serialErr.String())
	}
	var parOut, parErr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--jobs", "4", dir}, &parOut, &parErr); err != nil {
		t.Fatalf("parallel: %v\nstderr: %s", err, parErr.String())
	}
	if serialOut.String() != parOut.String() {
		t.Errorf("output drift between --jobs 1 and --jobs 4\n--- serial ---\n%s\n--- parallel ---\n%s",
			serialOut.String(), parOut.String())
	}
}

// TestLintCmd_JobsZeroRejected — a value of 0 has no semantic meaning
// (it would silently fall back to GOMAXPROCS), so reject it with a
// helpful error rather than treating it as the default.
func TestLintCmd_JobsZeroRejected(t *testing.T) {
	dir := makeMultiFileFixture(t, 1)
	var stdout, stderr bytes.Buffer
	err := run([]string{"lint", "--no-config", "--jobs", "0", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("--jobs 0 should error; stdout=%q", stdout.String())
	}
	if !strings.Contains(err.Error(), "jobs") {
		t.Errorf("error should mention jobs: %v", err)
	}
}

// TestLintCmd_JobsNonInteger rejects garbage values with a helpful
// message instead of silently treating them as zero.
func TestLintCmd_JobsNonInteger(t *testing.T) {
	dir := makeMultiFileFixture(t, 1)
	var stdout, stderr bytes.Buffer
	err := run([]string{"lint", "--no-config", "--jobs", "abc", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("--jobs abc should error")
	}
}

// TestLintCmd_CacheRoundtrip writes a cache file, runs the linter,
// then runs again with the same cache and confirms the output is
// identical. The first run populates the cache; the second hits it.
func TestLintCmd_CacheRoundtrip(t *testing.T) {
	dir := makeMultiFileFixture(t, 10)
	cachePath := filepath.Join(t.TempDir(), "lint.cache")

	var first, firstErr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--cache", cachePath, dir}, &first, &firstErr); err != nil {
		t.Fatalf("first run: %v\nstderr: %s", err, firstErr.String())
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file not created: %v", err)
	}

	var second, secondErr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--cache", cachePath, dir}, &second, &secondErr); err != nil {
		t.Fatalf("second run: %v\nstderr: %s", err, secondErr.String())
	}
	if first.String() != second.String() {
		t.Errorf("cache run drift\n--- first ---\n%s\n--- second ---\n%s",
			first.String(), second.String())
	}
}

// TestLintCmd_NoCacheOverridesCache verifies --no-cache disables the
// cache even when --cache PATH is also passed (later flag would be
// ambiguous; --no-cache should win regardless of order).
func TestLintCmd_NoCacheOverridesCache(t *testing.T) {
	dir := makeMultiFileFixture(t, 3)
	cachePath := filepath.Join(t.TempDir(), "lint.cache")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"lint", "--no-config", "--cache", cachePath, "--no-cache", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Errorf("--no-cache should leave cache file uncreated; stat err = %v", err)
	}
}
