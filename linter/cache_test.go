package linter

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCache_HitOnUnchangedFile verifies that a second LintFiles run
// over the same untouched file returns the cached result instead of
// re-linting.
func TestCache_HitOnUnchangedFile(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.gob")
	src := filepath.Join(dir, "x.py")
	if err := os.WriteFile(src, []byte("import os\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cache, err := OpenCache(cachePath, nil)
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	results := LintFiles([]string{src}, Config{}, LintOptions{Jobs: 1, Cache: cache})
	if len(results) != 1 || len(results[0].Diagnostics) != 1 {
		t.Fatalf("first run: want 1 diag, got %v", results)
	}

	// Same file, same cache, no mutation: should be a hit.
	abs, _ := filepath.Abs(src)
	if _, ok := cache.entries[abs]; !ok {
		t.Fatal("cache has no entry for src after first run")
	}
	info, _ := os.Stat(src)
	if got, ok := cache.lookup(src, info, Config{}); !ok || len(got) != 1 {
		t.Errorf("lookup miss after first run; got ok=%v len=%d", ok, len(got))
	}
}

// TestCache_MissOnMtimeBump confirms that touching the file
// invalidates the cache entry.
func TestCache_MissOnMtimeBump(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.py")
	if err := os.WriteFile(src, []byte("import os\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache, _ := OpenCache(filepath.Join(dir, "cache.gob"), nil)
	LintFiles([]string{src}, Config{}, LintOptions{Jobs: 1, Cache: cache})

	// Bump mtime by 2 seconds (avoid filesystem mtime resolution edge
	// cases). The size is unchanged, only the mtime should differ.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(src, future, future); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(src)
	if _, ok := cache.lookup(src, info, Config{}); ok {
		t.Error("lookup hit after mtime bump, want miss")
	}
}

// TestCache_MissOnConfigChange confirms that flipping --select
// produces a different config hash and therefore a cache miss.
func TestCache_MissOnConfigChange(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.py")
	if err := os.WriteFile(src, []byte("import os\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache, _ := OpenCache(filepath.Join(dir, "cache.gob"), nil)
	LintFiles([]string{src}, Config{}, LintOptions{Jobs: 1, Cache: cache})

	info, _ := os.Stat(src)
	other := Config{Select: []string{"F401"}}
	if _, ok := cache.lookup(src, info, other); ok {
		t.Error("lookup hit after config change, want miss")
	}
}

// TestCache_SaveLoadRoundtrip writes the cache to disk then opens it
// fresh and verifies entries survive.
func TestCache_SaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.gob")
	src := filepath.Join(dir, "x.py")
	if err := os.WriteFile(src, []byte("import os\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c1, _ := OpenCache(cachePath, nil)
	LintFiles([]string{src}, Config{}, LintOptions{Jobs: 1, Cache: c1})
	if err := c1.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	c2, err := OpenCache(cachePath, nil)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	info, _ := os.Stat(src)
	if got, ok := c2.lookup(src, info, Config{}); !ok || len(got) != 1 {
		t.Errorf("reopened cache: lookup ok=%v len=%d, want hit with 1 diag",
			ok, len(got))
	}
}

// TestCache_CorruptFile checks that a junk cache file doesn't crash
// — the warn callback fires and the cache starts empty.
func TestCache_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.gob")
	if err := os.WriteFile(cachePath, []byte("not gob"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warned string
	c, err := OpenCache(cachePath, func(s string) { warned = s })
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	if c == nil || len(c.entries) != 0 {
		t.Errorf("want empty cache after corrupt load, got entries=%v", c.entries)
	}
	if warned == "" {
		t.Error("warn callback never fired on corrupt cache")
	}
}

// TestCache_NilSafe verifies the nil-receiver methods used by
// lintOneFile when --cache wasn't passed.
func TestCache_NilSafe(t *testing.T) {
	var c *Cache
	if got, ok := c.lookup("x", nil, Config{}); ok || got != nil {
		t.Errorf("nil lookup returned %v, %v; want nil, false", got, ok)
	}
	c.store("x", nil, Config{}, nil) // must not panic
	if err := c.Save(); err != nil {
		t.Errorf("nil Save returned %v, want nil", err)
	}
}

// TestConfigHash_StableUnderOrder checks that reordering Select
// entries produces the same hash so cache hits don't depend on
// pyproject.toml line order.
func TestConfigHash_StableUnderOrder(t *testing.T) {
	a := Config{Select: []string{"F401", "F541"}}
	b := Config{Select: []string{"F541", "F401"}}
	if configHash(a) != configHash(b) {
		t.Errorf("hash differs under reorder: %s vs %s",
			configHash(a), configHash(b))
	}
}
