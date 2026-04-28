// Command gopapy parses Python source and emits an AST compatible with
// CPython's ast.dump output.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/cst"
	legacylinter "github.com/tamnd/gopapy/legacy/linter"
	legacyparser "github.com/tamnd/gopapy/legacy/parser"
	"github.com/tamnd/gopapy/lsp"

	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/linter"
	"github.com/tamnd/gopapy/parser"
	"github.com/tamnd/gopapy/symbols"
)

const version = "0.6.0"

func init() {
	// Mirror the CLI version into the LSP server so the initialize
	// response carries the same string `gopapy version` prints. Done
	// here rather than in lsp.Serve so the lsp package keeps zero
	// dependencies on the CLI.
	lsp.ServerVersion = version
}

func main() {
	if err := runWithStdin(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "gopapy:", err)
		os.Exit(1)
	}
}

// run is the test entry point that doesn't need stdin (every existing
// test passes paths). New stdin-aware tests call runWithStdin directly.
func run(args []string, stdout, stderr io.Writer) error {
	return runWithStdin(args, bytes.NewReader(nil), stdout, stderr)
}

func runWithStdin(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return nil
	}
	switch args[0] {
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, version)
		return nil
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	case "parse":
		if len(args) < 2 {
			return fmt.Errorf("parse: missing FILE argument")
		}
		_, err := parseFile2(args[1])
		return err
	case "dump":
		return dumpCmd(args[1:], stdout)
	case "unparse":
		return unparseCmd(args[1:], stdout, stderr)
	case "check":
		if len(args) < 2 {
			return fmt.Errorf("check: missing DIR argument")
		}
		return checkDir(args[1], stdout, stderr)
	case "symbols":
		if len(args) < 2 {
			return fmt.Errorf("symbols: missing PATH argument")
		}
		return symbolsCmd(args[1], stdout, stderr)
	case "diag":
		return diagCmd(args[1:], stdout, stderr)
	case "lint":
		return lintCmd(args[1:], stdin, stdout, stderr)
	case "lsp":
		return lspCmd(args[1:], stdin, stdout, stderr)
	case "bench":
		if len(args) < 2 {
			return fmt.Errorf("bench: missing DIR argument")
		}
		return benchCmd(args[1], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q (try 'gopapy help')", args[0])
	}
}

func parseFile2(path string) (*parser.Module, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(path, string(src))
}

// dumpCmd implements `gopapy dump [--py 3.X] FILE`.
// The --py flag records the target Python version for future version-aware
// dump output (Round 1.5.4); the flag is accepted now so callers can already
// pass it and the version table is exercised.
func dumpCmd(args []string, stdout io.Writer) error {
	pyVer := parser.LatestMinor
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--py", "--python-version":
			if i+1 >= len(args) {
				return fmt.Errorf("dump: %s requires a version argument (e.g. 3.12)", args[i])
			}
			minor, err := parser.ParseVersion(args[i+1])
			if err != nil {
				return fmt.Errorf("dump: %w", err)
			}
			pyVer = minor
			i++
		default:
			rest = append(rest, args[i])
		}
	}
	if len(rest) == 0 {
		return fmt.Errorf("dump: missing FILE argument")
	}
	src, err := os.ReadFile(rest[0])
	if err != nil {
		return err
	}
	m, err := parser.ParseFile(rest[0], string(src))
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, parser.ASTDump(m, pyVer))
	return nil
}

// checkDir walks DIR and reports parse outcomes for every .py file. The
// summary distinguishes successes from failures so the harness can be
// pointed at a corpus and produce a quick health number.
func checkDir(dir string, stdout, stderr io.Writer) error {
	var passed, failed int
	const gcEvery = 16
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".py") {
			return nil
		}
		if isIntentionalBadFixture(path) {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			failed++
			fmt.Fprintf(stderr, "FAIL %s: %v\n", path, readErr)
			return nil
		}
		if _, perr := parser.ParseFile(path, string(src)); perr != nil {
			failed++
			fmt.Fprintf(stderr, "FAIL %s: %v\n", path, perr)
		} else {
			passed++
		}
		// Free per-file parse trees promptly on large corpora.
		if (passed+failed)%gcEvery == 0 {
			runtime.GC()
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return fmt.Errorf("%d files failed to parse", failed)
	}
	return nil
}

// benchCmd walks a directory, parses every .py through the new parser
// pipeline, and reports throughput numbers against actual source bytes.
// Output is grep-friendly for diff-ing two runs or scraping by CI.
func benchCmd(dir string, stdout, stderr io.Writer) error {
	type entry struct {
		path string
		src  []byte
	}
	var files []entry
	var totalBytes int64
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(p, ".py") {
			return nil
		}
		if isIntentionalBadFixture(p) {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, entry{p, src})
		totalBytes += int64(len(src))
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("bench: no .py files under %s", dir)
	}

	var parseFailed int
	parseStart := time.Now()
	for _, f := range files {
		if _, err := parser.ParseFile(f.path, string(f.src)); err != nil {
			parseFailed++
			fmt.Fprintf(stderr, "FAIL %s: %v\n", f.path, err)
		}
	}
	parseDur := time.Since(parseStart)

	mb := float64(totalBytes) / (1024 * 1024)
	filesPerSec := float64(len(files)) / parseDur.Seconds()
	mbPerSec := mb / parseDur.Seconds()
	fmt.Fprintf(stdout, "corpus-files: %d\n", len(files))
	fmt.Fprintf(stdout, "corpus-bytes: %.1f MB\n", mb)
	fmt.Fprintf(stdout, "corpus-parse-rate: %.1f files/s, %.1f MB/s\n", filesPerSec, mbPerSec)
	if parseFailed > 0 {
		fmt.Fprintf(stdout, "corpus-parse-failed: %d\n", parseFailed)
	}
	return nil
}

// unparseCmd handles `gopapy unparse PATH`: parse, render via
// ast.Unparse, write to stdout. With `--check`, parse + unparse +
// re-parse + dump-compare each .py and exit non-zero if any file
// round-trips lossily; useful as a CI gate against the local stdlib.
func unparseCmd(args []string, stdout, stderr io.Writer) error {
	check := false
	comments := false
	allow := map[string]bool{}
	var path string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--check":
			check = true
		case a == "--comments":
			comments = true
		case a == "--allow":
			if i+1 >= len(args) {
				return fmt.Errorf("unparse: --allow requires a path")
			}
			i++
			abs, err := filepath.Abs(args[i])
			if err != nil {
				return err
			}
			allow[abs] = true
		case strings.HasPrefix(a, "--allow="):
			abs, err := filepath.Abs(strings.TrimPrefix(a, "--allow="))
			if err != nil {
				return err
			}
			allow[abs] = true
		default:
			if path != "" {
				return fmt.Errorf("unparse: unexpected argument %q", a)
			}
			path = a
		}
	}
	if path == "" {
		return fmt.Errorf("unparse: missing PATH argument")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return unparseOne(path, check, comments, stdout, stderr)
	}
	if !check {
		return fmt.Errorf("unparse: directory input requires --check")
	}
	var fileCount, parseFailed, mismatched, allowed int
	const gcEvery = 16
	walkErr := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(p, ".py") {
			return nil
		}
		if isIntentionalBadFixture(p) {
			return nil
		}
		fileCount++
		abs, _ := filepath.Abs(p)
		isAllowed := allow[abs]
		sink := stderr
		if isAllowed {
			sink = io.Discard
		}
		status := unparseOne(p, true, comments, io.Discard, sink)
		if status != nil {
			if isAllowed {
				allowed++
				return nil
			}
			if errors.Is(status, errParseFailed) {
				parseFailed++
			} else {
				mismatched++
			}
		}
		if fileCount%gcEvery == 0 {
			runtime.GC()
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	fmt.Fprintf(stderr, "%d files, %d parse-failed, %d round-trip mismatched, %d allowed\n",
		fileCount, parseFailed, mismatched, allowed)
	if mismatched > 0 || parseFailed > 0 {
		return fmt.Errorf("unparse --check: %d round-trip mismatches, %d parse failures",
			mismatched, parseFailed)
	}
	return nil
}

var errParseFailed = errors.New("parse failed")

func unparseOne(path string, check, comments bool, stdout, stderr io.Writer) error {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "FAIL read %s: %v\n", path, err)
		return errParseFailed
	}
	var mod *ast.Module
	var out string
	if comments {
		cf, err := cst.Parse(path, src)
		if err != nil {
			fmt.Fprintf(stderr, "FAIL parse %s: %v\n", path, err)
			return errParseFailed
		}
		mod = cf.AST
		out = cf.Unparse()
	} else {
		f, err := legacyparser.ParseString(path, string(src))
		if err != nil {
			fmt.Fprintf(stderr, "FAIL parse %s: %v\n", path, err)
			return errParseFailed
		}
		mod = ast.FromFile(f)
		out = ast.Unparse(mod)
	}
	if !check {
		fmt.Fprint(stdout, out)
		return nil
	}
	d1 := ast.Dump(mod)
	// Skip files whose AST already contains the parser's f-string/t-string
	// error markers — round-trip can't be a clean test of unparse when
	// the input itself wasn't parsed cleanly.
	if strings.Contains(d1, "<fstring-error:") || strings.Contains(d1, "<tstring-error:") {
		return nil
	}
	f2, err := legacyparser.ParseString(path, out)
	if err != nil {
		fmt.Fprintf(stderr, "FAIL reparse %s: %v\n", path, err)
		return fmt.Errorf("reparse failed")
	}
	mod2 := ast.FromFile(f2)
	d2 := ast.Dump(mod2)
	if d1 != d2 {
		fmt.Fprintf(stderr, "DIFF %s\n", path)
		return fmt.Errorf("round-trip dump mismatch")
	}
	return nil
}

// diagCmd parses one file or every .py under a directory, runs symbols
// to surface semantic diagnostics, and prints them in either the
// default `filename:line:col: severity[code]: message` form or one JSON
// object per line (`--json`). The exit code is 1 if any diagnostic at
// SeverityError was reported; warnings and hints never fail the run
// (matching the v0.1.7 spec contract that warnings are advisory). Parse
// errors do fail with exit 1, since a file that doesn't parse can't be
// analysed.
func diagCmd(args []string, stdout, stderr io.Writer) error {
	jsonOut := false
	var path string
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		default:
			if path != "" {
				return fmt.Errorf("diag: unexpected argument %q", a)
			}
			path = a
		}
	}
	if path == "" {
		return fmt.Errorf("diag: missing PATH argument")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	var (
		diagnostics []diag.Diagnostic
		fileCount   int
		parseFailed int
		errorCount  int
	)

	emit := func(d diag.Diagnostic) error {
		if d.Severity == diag.SeverityError {
			errorCount++
		}
		if jsonOut {
			b, err := json.Marshal(d)
			if err != nil {
				return err
			}
			fmt.Fprintln(stdout, string(b))
		} else {
			fmt.Fprintln(stdout, d.String())
		}
		return nil
	}

	process := func(p string) {
		fileCount++
		mod2, perr := parseFile2(p)
		if perr != nil {
			parseFailed++
			fmt.Fprintf(stderr, "FAIL parse %s: %v\n", p, perr)
			return
		}
		sm := symbols.Build(mod2)
		for _, d := range sm.Diagnostics {
			d.Filename = p
			diagnostics = append(diagnostics, d)
			_ = emit(d)
		}
	}

	if info.IsDir() {
		const gcEvery = 16
		walkErr := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !strings.HasSuffix(p, ".py") {
				return nil
			}
			if isIntentionalBadFixture(p) {
				return nil
			}
			process(p)
			if fileCount%gcEvery == 0 {
				runtime.GC()
			}
			return nil
		})
		if walkErr != nil {
			return walkErr
		}
		fmt.Fprintf(stderr, "%d files, %d parse-failed, %d diagnostics\n",
			fileCount, parseFailed, len(diagnostics))
		// In directory mode, parse failures are informational: parser2 does
		// not yet handle every Python 3.14 construct, so failing the whole
		// run would block corpus scanning. Only error-level diagnostics gate
		// the exit code in directory mode.
		if errorCount > 0 {
			return fmt.Errorf("diag: %d errors", errorCount)
		}
		return nil
	}

	process(path)

	if errorCount > 0 || parseFailed > 0 {
		return fmt.Errorf("diag: %d errors, %d parse failures", errorCount, parseFailed)
	}
	return nil
}

// lintCmd runs the pyflakes-style linter on FILE or every .py under
// DIR. Output mirrors diagCmd. Exit code follows the pyflakes
// convention: warnings never fail the run; only parse failures do.
//
// With --fix, each file is parsed, fixed (only codes enabled by the
// loaded config — F401 and F811-literal today), unparsed back to
// source via cst.Unparse, and rewritten in place atomically. The
// remaining diagnostics after the fix are emitted on stdout.
//
// --format chooses the diagnostic encoding: text (default), json
// (NDJSON, ruff-compatible flat schema), github (GH Actions workflow
// command lines for inline PR annotations), or sarif (a single SARIF
// 2.1.0 log document for code-scanning ingest). --output PATH writes
// the diagnostic stream to a file instead of stdout; "-" is stdout.
// The config-load and run-summary lines stay on stderr so machine
// consumers see only diagnostics on the chosen sink.
//
// --json is kept as a deprecated alias for --format json so v0.1.16
// scripts keep working; it now uses the flat schema documented in
// the v0.1.17 changelog.
//
// Configuration precedence: --config overrides discovery; --no-config
// skips discovery entirely. Default behaviour walks up from the first
// PATH argument looking for pyproject.toml.
//
// Stdin mode (PATH = "-"): the source body is read from stdin and
// linted as a single file. --stdin-filename PATH gives the buffer
// a logical name used for diagnostic filenames, per-file ignore
// matching, and config discovery (the path doesn't have to exist on
// disk). With --fix in stdin mode, the rewritten source goes to the
// configured sink and the remaining diagnostics go to stderr — the
// editor pipes stdout back into the buffer and renders stderr as
// underlines.

func lintCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	format := linter.FormatText
	fix := false
	noConfig := false
	configPath := ""
	outputPath := ""
	stdinFilename := ""
	jobs := 0       // 0 = default to GOMAXPROCS in linter.LintFiles
	cachePath := "" // "" = cache disabled
	noCache := false
	var path string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			format = linter.FormatJSON
		case a == "--fix":
			fix = true
		case a == "--no-config":
			noConfig = true
		case a == "--config":
			if i+1 >= len(args) {
				return fmt.Errorf("lint: --config requires a path")
			}
			i++
			configPath = args[i]
		case strings.HasPrefix(a, "--config="):
			configPath = strings.TrimPrefix(a, "--config=")
		case a == "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("lint: --format requires a value")
			}
			i++
			f, ferr := linter.ParseFormat(args[i])
			if ferr != nil {
				return fmt.Errorf("lint: %v", ferr)
			}
			format = f
		case strings.HasPrefix(a, "--format="):
			f, ferr := linter.ParseFormat(strings.TrimPrefix(a, "--format="))
			if ferr != nil {
				return fmt.Errorf("lint: %v", ferr)
			}
			format = f
		case a == "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("lint: --output requires a path")
			}
			i++
			outputPath = args[i]
		case strings.HasPrefix(a, "--output="):
			outputPath = strings.TrimPrefix(a, "--output=")
		case a == "--stdin-filename":
			if i+1 >= len(args) {
				return fmt.Errorf("lint: --stdin-filename requires a path")
			}
			i++
			stdinFilename = args[i]
		case strings.HasPrefix(a, "--stdin-filename="):
			stdinFilename = strings.TrimPrefix(a, "--stdin-filename=")
		case a == "--jobs":
			if i+1 >= len(args) {
				return fmt.Errorf("lint: --jobs requires a value")
			}
			i++
			n, jerr := parseJobs(args[i])
			if jerr != nil {
				return jerr
			}
			jobs = n
		case strings.HasPrefix(a, "--jobs="):
			n, jerr := parseJobs(strings.TrimPrefix(a, "--jobs="))
			if jerr != nil {
				return jerr
			}
			jobs = n
		case a == "--cache":
			// Bare --cache takes the next arg as the path unless it
			// looks like another flag, in which case fall back to the
			// default cache location.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				cachePath = args[i]
			} else {
				cachePath = linter.DefaultCachePath()
				if cachePath == "" {
					return fmt.Errorf("lint: --cache requires a path (UserCacheDir unavailable)")
				}
			}
		case strings.HasPrefix(a, "--cache="):
			cachePath = strings.TrimPrefix(a, "--cache=")
		case a == "--no-cache":
			noCache = true
		default:
			if path != "" {
				return fmt.Errorf("lint: unexpected argument %q", a)
			}
			path = a
		}
	}
	if path == "" {
		return fmt.Errorf("lint: missing PATH argument")
	}
	if noCache {
		cachePath = ""
	}

	if path == "-" {
		return lintStdin(stdin, stdinFilename, configPath, noConfig, fix,
			format, outputPath, stdout, stderr)
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	cfg, cfgPath, err := resolveLintConfig(path, configPath, noConfig)
	if err != nil {
		return err
	}
	if cfgPath != "" {
		fmt.Fprintf(stderr, "loaded config from %s\n", cfgPath)
	}

	sink, closeSink, err := openOutput(outputPath, stdout)
	if err != nil {
		return fmt.Errorf("lint: %v", err)
	}
	defer closeSink()

	var (
		diagnostics []diag.Diagnostic
		fileCount   int
		parseFailed int
		fixedFiles  int
	)

	emit := func(d diag.Diagnostic) error {
		// SARIF is a whole-document format; the per-diagnostic write
		// path can't emit it, so we collect into `diagnostics` above
		// and flush once at the end.
		if format == linter.FormatSARIF {
			return nil
		}
		return linter.WriteDiagnostic(sink, d, format)
	}

	process := func(p string) {
		fileCount++
		src, rerr := os.ReadFile(p)
		if rerr != nil {
			parseFailed++
			fmt.Fprintf(stderr, "FAIL read %s: %v\n", p, rerr)
			return
		}
		if fix {
			fixed, ferr := fixOne(p, src, toLegacyConfig(cfg))
			if ferr != nil {
				parseFailed++
				fmt.Fprintf(stderr, "FAIL fix %s: %v\n", p, ferr)
				return
			}
			if fixed > 0 {
				fixedFiles++
			}
			// Re-read so the remaining diagnostics reflect the fixed
			// source on disk rather than the pre-fix copy in memory.
			src, rerr = os.ReadFile(p)
			if rerr != nil {
				parseFailed++
				fmt.Fprintf(stderr, "FAIL re-read %s: %v\n", p, rerr)
				return
			}
		}
		ds, lerr := linter.LintFileWithConfig(p, src, cfg)
		if lerr != nil {
			parseFailed++
			fmt.Fprintf(stderr, "FAIL parse %s: %v\n", p, lerr)
			return
		}
		for _, d := range ds {
			diagnostics = append(diagnostics, d)
			_ = emit(d)
		}
	}

	if info.IsDir() {
		paths, walkErr := collectPyPaths(path)
		if walkErr != nil {
			return walkErr
		}
		if fix {
			// --fix mutates files; keep the serial loop so file-count
			// stats and the in-place rewrite stay simple. The cache
			// would be invalidated by every successful fix anyway.
			const gcEvery = 16
			for i, p := range paths {
				process(p)
				if (i+1)%gcEvery == 0 {
					runtime.GC()
				}
			}
		} else {
			cache, cerr := openLintCache(cachePath, stderr)
			if cerr != nil {
				return cerr
			}
			results := linter.LintFiles(paths, cfg, linter.LintOptions{
				Jobs:  jobs,
				Cache: cache,
			})
			for _, r := range results {
				fileCount++
				if r.Err != nil {
					parseFailed++
					fmt.Fprintf(stderr, "FAIL %s: %v\n", r.Path, r.Err)
					continue
				}
				for _, d := range r.Diagnostics {
					diagnostics = append(diagnostics, d)
					_ = emit(d)
				}
			}
			if cache != nil {
				if serr := cache.Save(); serr != nil {
					fmt.Fprintf(stderr, "warning: cache save: %v\n", serr)
				}
			}
		}
		fmt.Fprintf(stderr, "%d files, %d parse-failed, %d diagnostics",
			fileCount, parseFailed, len(diagnostics))
		if fix {
			fmt.Fprintf(stderr, ", %d files fixed", fixedFiles)
		}
		fmt.Fprintln(stderr)
	} else {
		process(path)
	}

	if format == linter.FormatSARIF {
		if err := linter.WriteSARIFLog(sink, diagnostics, sarifTool()); err != nil {
			return fmt.Errorf("lint: write sarif: %v", err)
		}
	}

	if parseFailed > 0 {
		return fmt.Errorf("lint: %d parse failures", parseFailed)
	}
	return nil
}

// lspCmd runs the language-server loop on stdio. The editor manages
// the lifecycle: starts the process when a Python buffer opens in a
// configured workspace, sends shutdown+exit on close. No flags — the
// transport is fixed at stdio with LSP framing, and configuration
// discovery uses the same pyproject.toml walk the rest of the CLI
// does.
func lspCmd(args []string, stdin io.Reader, stdout, _ io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("lsp: takes no arguments, got %q", args[0])
	}
	return lsp.Serve(stdin, stdout)
}

// sarifTool builds the SARIF tool descriptor from the current build's
// version constant. Centralised so stdin-mode and dir-mode write the
// same `tool.driver` block.
func sarifTool() linter.ToolInfo {
	return linter.ToolInfo{
		Name:           "gopapy",
		Version:        version,
		InformationURI: "https://github.com/tamnd/gopapy",
	}
}

// lintStdin handles `gopapy lint -`. The source body is read from
// stdin to EOF and treated as a single file whose logical name is
// stdinFilename (default "<stdin>"). Per-file ignores and config
// discovery key off that name; with --fix, the rewritten source
// goes to the configured sink and the diagnostics go to stderr so
// an editor can pipe one stream into the buffer and render the
// other as squiggles.
func lintStdin(stdin io.Reader, stdinFilename, configPath string, noConfig, fix bool,
	format linter.Format, outputPath string, stdout, stderr io.Writer,
) error {
	src, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("lint: read stdin: %v", err)
	}
	logical := stdinFilename
	if logical == "" {
		logical = "<stdin>"
	}
	// Discovery anchor: the user-supplied path if any, else the CWD.
	// A real filename gets us into the right project; without one we
	// fall back to "where the user invoked us from".
	anchor := stdinFilename
	if anchor == "" {
		anchor = "."
	}
	cfg, cfgPath, err := resolveLintConfig(anchor, configPath, noConfig)
	if err != nil {
		return err
	}
	if cfgPath != "" {
		fmt.Fprintf(stderr, "loaded config from %s\n", cfgPath)
	}

	sink, closeSink, err := openOutput(outputPath, stdout)
	if err != nil {
		return fmt.Errorf("lint: %v", err)
	}
	defer closeSink()

	if fix {
		// In stdin --fix mode the sink receives source bytes; the
		// diagnostic stream moves to stderr so callers can split the
		// two cleanly.
		out, fixedDiags, ferr := fixStdin(logical, src, toLegacyConfig(cfg))
		if ferr != nil {
			return fmt.Errorf("lint: parse stdin: %v", ferr)
		}
		if _, err := sink.Write([]byte(out)); err != nil {
			return fmt.Errorf("lint: write fixed source: %v", err)
		}
		// After applying the fix we lint the rewritten body so the
		// diagnostics reflect what the buffer will actually contain.
		ds, derr := linter.LintFileWithConfig(logical, []byte(out), cfg)
		if derr != nil {
			return fmt.Errorf("lint: re-lint stdin: %v", derr)
		}
		if format == linter.FormatSARIF {
			if err := linter.WriteSARIFLog(stderr, ds, sarifTool()); err != nil {
				return fmt.Errorf("lint: write sarif: %v", err)
			}
		} else {
			for _, d := range ds {
				_ = linter.WriteDiagnostic(stderr, d, format)
			}
		}
		fmt.Fprintf(stderr, "stdin: %d diagnostics, %d fixes applied\n",
			len(ds), len(fixedDiags))
		return nil
	}

	ds, derr := linter.LintFileWithConfig(logical, src, cfg)
	if derr != nil {
		return fmt.Errorf("lint: parse stdin: %v", derr)
	}
	if format == linter.FormatSARIF {
		if err := linter.WriteSARIFLog(sink, ds, sarifTool()); err != nil {
			return fmt.Errorf("lint: write sarif: %v", err)
		}
		return nil
	}
	for _, d := range ds {
		_ = linter.WriteDiagnostic(sink, d, format)
	}
	return nil
}

// toLegacyConfig converts the new linter.Config into the legacy
// equivalent so the --fix path (which still runs against the v1 AST
// fixers) can consume it without re-encoding.
func toLegacyConfig(c linter.Config) legacylinter.Config {
	return legacylinter.Config{
		Select:  c.Select,
		Ignore:  c.Ignore,
		PerFile: c.PerFile,
	}
}

// fixStdin parses src, applies safe fixes, and returns the rewritten
// body plus the list of fixed diagnostics. Mirrors fixOne minus the
// disk dance: there's no path to atomic-rename onto.
func fixStdin(filename string, src []byte, cfg legacylinter.Config) (string, []legacylinter.FixedDiagnostic, error) {
	cf, err := cst.Parse(filename, src)
	if err != nil {
		return "", nil, err
	}
	_, fixed := legacylinter.FixWithConfig(cf.AST, cfg, filename)
	if len(fixed) == 0 {
		return string(src), nil, nil
	}
	return cf.Unparse(), fixed, nil
}

// fixOne reads, parses, fixes, and re-emits one file. Returns the
// number of fixes applied; 0 means the file is left untouched on
// disk. The write goes through a temp file in the same directory
// and a rename so a crash mid-write can't truncate the source.
//
// Codes ignored by cfg (globally or per-file) are skipped inside
// legacylinter.FixWithConfig, so the on-disk file stays consistent with
// the diagnostics the user is allowed to see.
func fixOne(path string, src []byte, cfg legacylinter.Config) (int, error) {
	cf, err := cst.Parse(path, src)
	if err != nil {
		return 0, err
	}
	_, fixed := legacylinter.FixWithConfig(cf.AST, cfg, path)
	if len(fixed) == 0 {
		return 0, nil
	}
	out := cf.Unparse()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gopapy-fix-*.py")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return 0, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return 0, err
	}
	return len(fixed), nil
}

// openOutput resolves --output to a writer. Empty path or "-" means
// stdout (no close); a real path opens for write+truncate. Caller
// must defer the returned close func; it's a no-op for stdout.
func openOutput(path string, stdout io.Writer) (io.Writer, func(), error) {
	if path == "" || path == "-" {
		return stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
}

// resolveLintConfig loads the config that should govern this lint
// invocation. Precedence: --no-config -> zero Config; --config PATH ->
// LoadConfig(PATH); otherwise discover by walking up from the user-
// supplied path. The returned cfgPath is non-empty only when a file
// was actually loaded, so the caller can echo "loaded config from X"
// to stderr without false positives.
// parseJobs converts a CLI --jobs value into the int LintOptions
// expects. Zero is rejected (no semantic meaning); negative values
// are rejected too.
func parseJobs(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("lint: --jobs %q: %v", s, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("lint: --jobs must be >= 1, got %d", n)
	}
	return n, nil
}

// collectPyPaths walks dir and returns every .py path in lexical
// order, skipping intentional bad-syntax fixtures. Sorted so
// LintFiles emits output in the same order single-threaded
// filepath.WalkDir does.
func collectPyPaths(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return werr
		}
		if !strings.HasSuffix(p, ".py") {
			return nil
		}
		if isIntentionalBadFixture(p) {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// openLintCache resolves cachePath into a *linter.Cache, returning
// nil when caching is disabled (empty path). A corrupt cache file
// produces a warning on stderr and an empty cache; the lint run
// continues either way.
func openLintCache(cachePath string, stderr io.Writer) (*linter.Cache, error) {
	if cachePath == "" {
		return nil, nil
	}
	warn := func(msg string) { fmt.Fprintln(stderr, "warning:", msg) }
	c, err := linter.OpenCache(cachePath, warn)
	if err != nil {
		return nil, fmt.Errorf("lint: open cache %s: %v", cachePath, err)
	}
	return c, nil
}

func resolveLintConfig(walkFrom, configPath string, noConfig bool) (linter.Config, string, error) {
	if noConfig {
		return linter.Config{}, "", nil
	}
	if configPath != "" {
		cfg, err := linter.LoadConfig(configPath)
		if err != nil {
			return linter.Config{}, "", err
		}
		return cfg, configPath, nil
	}
	return linter.DiscoverConfig(walkFrom)
}

// symbolsCmd builds a symbol table for one file or every .py under a
// directory. The contract from the v0.1.4 spec is "Build never panics
// on stdlib"; semantic warnings on individual files do not fail the run.
// Parse failures are still counted (a file that won't parse can't have
// its symbols built), and Build panics propagate as test failures.
func symbolsCmd(path string, stdout, stderr io.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		mod2, err := parseFile2(path)
		if err != nil {
			return err
		}
		sm := symbols.Build(mod2)
		printSymbolModule(stdout, path, sm)
		return nil
	}
	var passed, parseFailed, panicked int
	const gcEvery = 16
	walkErr := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(p, ".py") {
			return nil
		}
		if isIntentionalBadFixture(p) {
			return nil
		}
		mod2, perr := parseFile2(p)
		if perr != nil {
			parseFailed++
			return nil
		}
		if err := buildSymbolsSafe2(mod2); err != nil {
			panicked++
			fmt.Fprintf(stderr, "PANIC %s: %v\n", p, err)
		} else {
			passed++
		}
		if (passed+parseFailed+panicked)%gcEvery == 0 {
			runtime.GC()
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	fmt.Fprintf(stdout, "%d passed, %d parse-failed, %d panicked\n", passed, parseFailed, panicked)
	if panicked > 0 {
		return fmt.Errorf("%d files panicked in symbols.Build", panicked)
	}
	return nil
}

func buildSymbolsSafe2(mod *parser.Module) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	_ = symbols.Build(mod)
	return nil
}

// printSymbolModule writes a compact one-scope-per-line dump for a single
// file invocation. The format prioritises being grep-friendly over being
// pretty.
func printSymbolModule(w io.Writer, path string, mod *symbols.Module) {
	fmt.Fprintf(w, "# %s\n", path)
	var walk func(s *symbols.Scope, depth int)
	walk = func(s *symbols.Scope, depth int) {
		indent := strings.Repeat("  ", depth)
		fmt.Fprintf(w, "%s%s %q\n", indent, s.Kind, s.Name)
		for name, sym := range s.Symbols {
			fmt.Fprintf(w, "%s  %s flags=%s\n", indent, name, flagString(sym.Flags))
		}
		for _, c := range s.Children {
			walk(c, depth+1)
		}
	}
	walk(mod.Root, 0)
	for _, d := range mod.Diagnostics {
		fmt.Fprintf(w, "  diag %d:%d: %s\n", d.Pos.Line, d.Pos.Col, d.Msg)
	}
}

// flagString renders a BindFlag as a comma-separated list. Unflagged
// names render as "-".
func flagString(f symbols.BindFlag) string {
	parts := []string{}
	pairs := []struct {
		f symbols.BindFlag
		s string
	}{
		{symbols.FlagBound, "bound"},
		{symbols.FlagUsed, "used"},
		{symbols.FlagParam, "param"},
		{symbols.FlagGlobal, "global"},
		{symbols.FlagNonlocal, "nonlocal"},
		{symbols.FlagAnnotation, "ann"},
		{symbols.FlagImport, "import"},
		{symbols.FlagFree, "free"},
		{symbols.FlagCell, "cell"},
	}
	for _, p := range pairs {
		if f&p.f != 0 {
			parts = append(parts, p.s)
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

// isIntentionalBadFixture reports whether path is a fixture that is meant to
// be unparseable. Covers CPython's bad_*/badsyntax_* naming convention and
// known intentionally-invalid files from the PyPI corpus packages.
func isIntentionalBadFixture(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, "bad_") || strings.HasPrefix(base, "badsyntax_") {
		return true
	}
	// Intentionally-invalid corpus fixtures (both gopapy and CPython reject them).
	switch base {
	case "async_as_identifier.py",   // black: async used as identifier (Python 2 style)
		"invalid_header.py",         // black: malformed encoding declaration
		"pattern_matching_invalid.py", // black: walrus inside match pattern
		"python2_detection.py",      // black: Python 2 print statement
		"pep_572_do_not_remove_parens.py", // black: invalid walrus target
		"pep_701.py",                // black: intentionally-broken f-string nesting
		"tests_syntax_error.py",     // Django: deliberate SyntaxError fixture
		"unicodedoc.py":             // pygments: uses ur"..." (invalid in Python 3)
		return true
	}
	return false
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: gopapy <command> [args]

Commands:
  parse FILE    Parse a Python source file; exit non-zero on error.
  dump  FILE    Print AST in ast.dump style.
  unparse FILE  Round-trip the file through Unparse and print the result.
  check DIR     Parse every .py under DIR, summarise failures.
  symbols PATH  Build a symbol table for FILE, or report panics across DIR.
  diag PATH     Print semantic diagnostics for FILE or DIR. --json for JSONL.
  lint PATH     Run pyflakes-style linter on FILE or DIR.
                PATH = "-" reads source from stdin (use --stdin-filename
                to give the buffer a logical name for diagnostics and
                per-file ignores).
                --format {text,json,github,sarif} chooses the diagnostic encoding.
                --output PATH writes diagnostics to a file ("-" = stdout).
                --fix rewrites files in place (F401, F811 dead-store);
                in stdin mode --fix writes the rewritten source to the
                output sink and the diagnostics to stderr.
                --config PATH / --no-config control pyproject.toml discovery.
                --jobs N runs the file pool with N workers (default GOMAXPROCS).
                --cache [PATH] enables an opt-in result cache keyed on
                (path, mtime, size, config-hash). Default location is
                $XDG_CACHE_HOME/gopapy/lint.cache.
                --no-cache disables caching for this run.
  lsp           Run the language-server loop on stdio. Editor-only;
                publishes diagnostics on textDocument/didOpen and
                textDocument/didChange (full-content sync).
  bench DIR     Parse every .py under DIR and print parse/emit throughput.
  version       Print the gopapy version.
  help          Show this message.
`)
}
