package linter

import (
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/tamnd/gopapy/diag"
)

// FileResult is the per-file output of LintFiles.
type FileResult struct {
	Path        string
	Diagnostics []diag.Diagnostic
	Err         error
}

// LintOptions controls parallel execution.
type LintOptions struct {
	Jobs  int
	Cache *Cache
}

// LintFileWithConfig runs LintFile and applies cfg's filtering.
func LintFileWithConfig(filename string, src []byte, cfg Config) ([]diag.Diagnostic, error) {
	all, err := LintFile(filename, src)
	if err != nil {
		return nil, err
	}
	out := make([]diag.Diagnostic, 0, len(all))
	for _, d := range all {
		if !cfg.EnabledFor(filename, d.Code) {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// LintFiles lints every path in paths under cfg and returns one
// FileResult per path in input order.
func LintFiles(paths []string, cfg Config, opts LintOptions) []FileResult {
	if len(paths) == 0 {
		return nil
	}
	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = runtime.GOMAXPROCS(0)
	}
	if jobs > len(paths) {
		jobs = len(paths)
	}

	results := make([]FileResult, len(paths))
	for i, p := range paths {
		results[i].Path = p
	}

	if jobs == 1 {
		for i, p := range paths {
			results[i] = lintOneFile(p, cfg, opts.Cache)
		}
		return results
	}

	type job struct{ idx int }
	ch := make(chan job, len(paths))
	for i := range paths {
		ch <- job{idx: i}
	}
	close(ch)

	var wg sync.WaitGroup
	wg.Add(jobs)
	for w := 0; w < jobs; w++ {
		go func() {
			defer wg.Done()
			for j := range ch {
				results[j.idx] = lintOneFile(paths[j.idx], cfg, opts.Cache)
			}
		}()
	}
	wg.Wait()
	return results
}

func lintOneFile(path string, cfg Config, cache *Cache) FileResult {
	r := FileResult{Path: path}
	info, statErr := os.Stat(path)
	if statErr != nil {
		r.Err = statErr
		return r
	}
	if cache != nil {
		if ds, ok := cache.Lookup(path, info, cfg); ok {
			r.Diagnostics = ds
			return r
		}
	}
	src, readErr := os.ReadFile(path)
	if readErr != nil {
		r.Err = readErr
		return r
	}
	ds, lerr := LintFileWithConfig(path, src, cfg)
	if lerr != nil {
		r.Err = lerr
		return r
	}
	r.Diagnostics = ds
	if cache != nil {
		cache.Store(path, info, cfg, ds)
	}
	return r
}

// SortPaths returns a copy of paths sorted lexically.
func SortPaths(paths []string) []string {
	out := make([]string, len(paths))
	copy(out, paths)
	sort.Strings(out)
	return out
}
