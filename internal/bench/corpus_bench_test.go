// Package bench provides corpus-scale parser benchmarks.
//
// Unlike the small fixtures in parser/bench_test.go, these benchmarks
// pre-load real corpus files from corpus/src/ and measure ParseFile
// throughput without disk I/O noise. They are the authoritative numbers
// for tracking progress toward the 100x CPython goal.
//
// Run:
//
//	go test -bench=BenchmarkCorpus -benchtime=5s -benchmem ./internal/bench/
//
// The benchmark skips gracefully when corpus/src/ is not present (CI
// corpus cache miss or local development without a downloaded corpus).
package bench

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tamnd/gopapy/parser"
)

const (
	// maxFiles caps how many files are pre-loaded so the benchmark does
	// not exhaust RAM on machines with small corpora.
	maxFiles = 4000
	corpusDir = "../../corpus/src"
)

type corpusFile struct {
	path string
	src  string
}

// loadCorpus walks corpusDir and pre-reads up to maxFiles .py files.
// Returns nil, 0 when the directory does not exist.
func loadCorpus() ([]corpusFile, int64) {
	var files []corpusFile
	var totalBytes int64

	_ = filepath.WalkDir(corpusDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".py" {
			return nil
		}
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		files = append(files, corpusFile{path: path, src: string(data)})
		totalBytes += int64(len(data))
		return nil
	})

	return files, totalBytes
}

// BenchmarkCorpusParseSerial parses corpus files one at a time in a
// single goroutine. This is the baseline for measuring per-optimization
// speedup independent of parallelism.
func BenchmarkCorpusParseSerial(b *testing.B) {
	files, totalBytes := loadCorpus()
	if len(files) == 0 {
		b.Skip("corpus/src not found; run corpus/download.sh first")
	}

	// Each b.Loop() iteration processes all files; set total bytes so
	// the reported MB/s reflects aggregate throughput.
	b.SetBytes(totalBytes)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		for _, f := range files {
			if _, err := parser.ParseFile(f.path, f.src); err != nil {
				// Parse errors for intentionally-invalid fixtures are expected.
				_ = err
			}
		}
	}
}

// BenchmarkCorpusParseParallel parses corpus files across runtime.NumCPU()
// goroutines to measure the upper bound of parallel throughput.
// Introduced in v0.6.1; included here so the benchmark can run before
// parallel processing is wired into the CLI.
func BenchmarkCorpusParseParallel(b *testing.B) {
	files, totalBytes := loadCorpus()
	if len(files) == 0 {
		b.Skip("corpus/src not found; run corpus/download.sh first")
	}

	workers := runtime.NumCPU()
	b.SetBytes(totalBytes)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		ch := make(chan corpusFile, workers*2)
		done := make(chan struct{})

		for range workers {
			go func() {
				for f := range ch {
					_, _ = parser.ParseFile(f.path, f.src)
				}
				done <- struct{}{}
			}()
		}

		for _, f := range files {
			ch <- f
		}
		close(ch)

		for range workers {
			<-done
		}
	}
}
