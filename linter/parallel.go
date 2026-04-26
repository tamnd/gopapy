package linter

import (
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/tamnd/gopapy/v1/diag"
)

// FileResult is the per-file output of LintFiles. Diagnostics is the
// list LintFileWithConfig returned for the file; Err is set when the
// file couldn't be read or parsed. The two are mutually exclusive: a
// successful lint sets Err to nil even if Diagnostics is empty.
type FileResult struct {
	Path        string
	Diagnostics []diag.Diagnostic
	Err         error
}

// LintOptions controls parallel execution. The zero value runs with
// runtime.GOMAXPROCS(0) workers and no cache.
type LintOptions struct {
	// Jobs is the worker-pool size. <= 0 means GOMAXPROCS(0). 1 forces
	// serial execution. The CLI rejects 0 at parse time so users get a
	// clear error rather than silent default-application.
	Jobs int

	// Cache, if non-nil, is consulted before parsing each file and
	// updated after a successful lint. Safe for concurrent use.
	Cache *Cache
}

// LintFiles lints every path in paths under cfg and returns one
// FileResult per path in input order. The output is deterministic for
// a given (paths, cfg) pair regardless of Jobs. Pass paths in the
// order you want results emitted; the CLI sorts before calling so
// `gopapy lint dir/` produces lexical-walk output.
//
// Each worker reads the file, calls LintFileWithConfig, and pushes
// the result back. Errors are per-file: a parse failure on file N
// doesn't stop work on file N+1.
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

// lintOneFile is the per-file unit of work shared by serial and
// parallel paths. Reads the file, optionally consults the cache,
// runs the lint, and writes back into the cache on success.
func lintOneFile(path string, cfg Config, cache *Cache) FileResult {
	r := FileResult{Path: path}
	info, statErr := os.Stat(path)
	if statErr != nil {
		r.Err = statErr
		return r
	}
	if cache != nil {
		if ds, ok := cache.lookup(path, info, cfg); ok {
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
		cache.store(path, info, cfg, ds)
	}
	return r
}

// SortPaths returns a copy of paths sorted lexically. Helper for
// callers that want the deterministic-output guarantee from LintFiles
// but haven't sorted their input yet.
func SortPaths(paths []string) []string {
	out := make([]string, len(paths))
	copy(out, paths)
	sort.Strings(out)
	return out
}
