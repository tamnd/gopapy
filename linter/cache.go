package linter

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/tamnd/gopapy/diag"
)

// Cache is an opt-in result cache keyed on (absolute path, mtime,
// size, config hash). Safe for concurrent use by LintFiles workers.
type Cache struct {
	path string

	mu      sync.Mutex
	entries map[string]cacheEntry
	dirty   bool
}

type cacheEntry struct {
	ConfigHash  string
	Mtime       time.Time
	Size        int64
	Diagnostics []diag.Diagnostic
}

// OpenCache loads a cache from path. A missing file is not an error.
// A corrupt file is logged via warn and the cache starts empty.
func OpenCache(path string, warn func(string)) (*Cache, error) {
	c := &Cache{path: path, entries: map[string]cacheEntry{}}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	defer f.Close()
	var loaded map[string]cacheEntry
	if derr := gob.NewDecoder(f).Decode(&loaded); derr != nil {
		if warn != nil {
			warn(fmt.Sprintf("cache %s: corrupt (%v); starting empty", path, derr))
		}
		return c, nil
	}
	c.entries = loaded
	return c, nil
}

// Save writes the cache to disk via tmp-then-rename.
func (c *Cache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if eerr := gob.NewEncoder(f).Encode(c.entries); eerr != nil {
		f.Close()
		os.Remove(tmp)
		return eerr
	}
	if cerr := f.Close(); cerr != nil {
		os.Remove(tmp)
		return cerr
	}
	if rerr := os.Rename(tmp, c.path); rerr != nil {
		os.Remove(tmp)
		return rerr
	}
	c.dirty = false
	return nil
}

// Lookup returns cached diagnostics for path if the (mtime, size,
// config-hash) tuple matches. Returns (nil, false) on any miss.
func (c *Cache) Lookup(path string, info os.FileInfo, cfg Config) ([]diag.Diagnostic, bool) {
	if c == nil {
		return nil, false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[abs]
	if !ok {
		return nil, false
	}
	if e.ConfigHash != configHash(cfg) {
		return nil, false
	}
	if !e.Mtime.Equal(info.ModTime()) || e.Size != info.Size() {
		return nil, false
	}
	out := make([]diag.Diagnostic, len(e.Diagnostics))
	copy(out, e.Diagnostics)
	return out, true
}

// Store records post-lint diagnostics under the current key.
func (c *Cache) Store(path string, info os.FileInfo, cfg Config, ds []diag.Diagnostic) {
	if c == nil {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	saved := make([]diag.Diagnostic, len(ds))
	copy(saved, ds)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[abs] = cacheEntry{
		ConfigHash:  configHash(cfg),
		Mtime:       info.ModTime(),
		Size:        info.Size(),
		Diagnostics: saved,
	}
	c.dirty = true
}

// configHash is sha256 of cfg with all slice fields sorted.
func configHash(cfg Config) string {
	norm := struct {
		Select  []string
		Ignore  []string
		PerFile map[string][]string
	}{
		Select:  sortedCopy(cfg.Select),
		Ignore:  sortedCopy(cfg.Ignore),
		PerFile: map[string][]string{},
	}
	for k, v := range cfg.PerFile {
		norm.PerFile[k] = sortedCopy(v)
	}
	b, err := json.Marshal(norm)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sortedCopy(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}

// DefaultCachePath is ~/.cache/gopapy/lint.cache.
func DefaultCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "gopapy", "lint.cache")
}
