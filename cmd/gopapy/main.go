// Command gopapy parses Python source and emits an AST compatible with
// CPython's ast.dump output.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tamnd/gopapy/v1/ast"
	"github.com/tamnd/gopapy/v1/diag"
	"github.com/tamnd/gopapy/v1/parser"
	"github.com/tamnd/gopapy/v1/symbols"
)

const version = "0.1.7"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "gopapy:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
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
		_, err := parseFile(args[1])
		return err
	case "dump":
		if len(args) < 2 {
			return fmt.Errorf("dump: missing FILE argument")
		}
		f, err := parseFile(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, ast.Dump(ast.FromFile(f)))
		return nil
	case "unparse":
		if len(args) < 2 {
			return fmt.Errorf("unparse: missing FILE argument")
		}
		f, err := parseFile(args[1])
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, ast.Unparse(ast.FromFile(f)))
		return nil
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
	case "bench":
		if len(args) < 2 {
			return fmt.Errorf("bench: missing DIR argument")
		}
		return benchCmd(args[1], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q (try 'gopapy help')", args[0])
	}
}

func parseFile(path string) (*parser.File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(path, src)
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
		if _, perr := parseFile(path); perr != nil {
			failed++
			fmt.Fprintf(stderr, "FAIL %s: %v\n", path, perr)
		} else {
			passed++
		}
		// Free per-file parse trees promptly. participle holds a lot of
		// transient state per file; without periodic GC the resident set
		// climbs into multi-GB territory on a 1800-file corpus.
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

// benchCmd walks a directory, parses every .py through the full
// pipeline (lex + participle + emit), and reports throughput numbers
// against actual source bytes. Useful for one-shot benchmarks against
// a corpus that isn't in our fixtures — e.g. someone's monorepo. The
// output format is grep-friendly so a wrapping script can diff two
// runs.
func benchCmd(dir string, stdout, stderr io.Writer) error {
	type entry struct {
		path string
		src  []byte
	}
	var files []entry
	var bytes int64
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
		bytes += int64(len(src))
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("bench: no .py files under %s", dir)
	}
	parseStart := time.Now()
	parsed := make([]*parser.File, 0, len(files))
	var parseFailed int
	for i, f := range files {
		pf, err := parser.ParseFile(f.path, f.src)
		if err != nil {
			parseFailed++
			parsed = append(parsed, nil)
			fmt.Fprintf(stderr, "FAIL parse %s: %v\n", f.path, err)
		} else {
			parsed = append(parsed, pf)
		}
		if (i+1)%64 == 0 {
			runtime.GC()
		}
	}
	parseDur := time.Since(parseStart)

	emitStart := time.Now()
	for _, pf := range parsed {
		if pf == nil {
			continue
		}
		_ = ast.FromFile(pf)
	}
	emitDur := time.Since(emitStart)

	mb := float64(bytes) / (1024 * 1024)
	fmt.Fprintf(stdout, "files: %d\n", len(files))
	fmt.Fprintf(stdout, "bytes: %.1f MB\n", mb)
	fmt.Fprintf(stdout, "parse: %s (%.2f MB/s)\n", parseDur.Round(time.Millisecond), mb/parseDur.Seconds())
	fmt.Fprintf(stdout, "emit:  %s (%.2f MB/s)\n", emitDur.Round(time.Millisecond), mb/emitDur.Seconds())
	fmt.Fprintf(stdout, "total: %s\n", (parseDur + emitDur).Round(time.Millisecond))
	if parseFailed > 0 {
		fmt.Fprintf(stdout, "parse-failed: %d\n", parseFailed)
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
		f, perr := parseFile(p)
		if perr != nil {
			parseFailed++
			fmt.Fprintf(stderr, "FAIL parse %s: %v\n", p, perr)
			return
		}
		mod := symbols.Build(ast.FromFile(f))
		for _, d := range mod.Diagnostics {
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
	} else {
		process(path)
	}

	if errorCount > 0 || parseFailed > 0 {
		return fmt.Errorf("diag: %d errors, %d parse failures", errorCount, parseFailed)
	}
	return nil
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
		f, err := parseFile(path)
		if err != nil {
			return err
		}
		mod := symbols.Build(ast.FromFile(f))
		printSymbolModule(stdout, path, mod)
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
		f, perr := parseFile(p)
		if perr != nil {
			parseFailed++
			return nil
		}
		if err := buildSymbolsSafe(ast.FromFile(f)); err != nil {
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

// buildSymbolsSafe runs symbols.Build with panic recovery so the harness
// can keep going across a 1800-file corpus and report every offender.
func buildSymbolsSafe(mod *ast.Module) (err error) {
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
		fmt.Fprintf(w, "  diag %d:%d: %s\n", d.Pos.Lineno, d.Pos.ColOffset, d.Msg)
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

// isIntentionalBadFixture reports whether path is a CPython test fixture
// that is *meant* to be unparseable. The naming convention `bad_*.py` /
// `badsyntax_*.py` is used by CPython's own tokenizer/parser tests to ship
// deliberate syntax errors.
func isIntentionalBadFixture(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "bad_") || strings.HasPrefix(base, "badsyntax_")
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
  bench DIR     Parse every .py under DIR and print parse/emit throughput.
  version       Print the gopapy version.
  help          Show this message.
`)
}
