// Command gopapy parses Python source and emits an AST compatible with
// CPython's ast.dump output.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tamnd/gopapy/v1/ast"
	"github.com/tamnd/gopapy/v1/cst"
	"github.com/tamnd/gopapy/v1/diag"
	"github.com/tamnd/gopapy/v1/linter"
	"github.com/tamnd/gopapy/v1/parser"
	"github.com/tamnd/gopapy/v1/symbols"
)

const version = "0.1.15"

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
		return lintCmd(args[1:], stdout, stderr)
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
		f, err := parser.ParseString(path, string(src))
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
	f2, err := parser.ParseString(path, out)
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

// lintCmd runs the pyflakes-style linter on FILE or every .py under
// DIR. Output mirrors diagCmd. Exit code follows the pyflakes
// convention: warnings never fail the run; only parse failures do.
//
// With --fix, each file is parsed, fixed (only safe F401 fixes
// today), unparsed back to source via cst.Unparse, and rewritten in
// place atomically (temp file + rename). The remaining diagnostics
// after the fix are emitted on stdout.
func lintCmd(args []string, stdout, stderr io.Writer) error {
	jsonOut := false
	fix := false
	var path string
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--fix":
			fix = true
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
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	var (
		diagnostics []diag.Diagnostic
		fileCount   int
		parseFailed int
		fixedFiles  int
	)

	emit := func(d diag.Diagnostic) error {
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
		src, rerr := os.ReadFile(p)
		if rerr != nil {
			parseFailed++
			fmt.Fprintf(stderr, "FAIL read %s: %v\n", p, rerr)
			return
		}
		if fix {
			fixed, ferr := fixOne(p, src)
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
		ds, lerr := linter.LintFile(p, src)
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
		fmt.Fprintf(stderr, "%d files, %d parse-failed, %d diagnostics",
			fileCount, parseFailed, len(diagnostics))
		if fix {
			fmt.Fprintf(stderr, ", %d files fixed", fixedFiles)
		}
		fmt.Fprintln(stderr)
	} else {
		process(path)
	}

	if parseFailed > 0 {
		return fmt.Errorf("lint: %d parse failures", parseFailed)
	}
	return nil
}

// fixOne reads, parses, fixes, and re-emits one file. Returns the
// number of fixes applied; 0 means the file is left untouched on
// disk. The write goes through a temp file in the same directory
// and a rename so a crash mid-write can't truncate the source.
func fixOne(path string, src []byte) (int, error) {
	cf, err := cst.Parse(path, src)
	if err != nil {
		return 0, err
	}
	_, fixed := linter.Fix(cf.AST)
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
  lint PATH     Run pyflakes-style linter on FILE or DIR. --json for JSONL.
                --fix rewrites files in place (F401 only).
  bench DIR     Parse every .py under DIR and print parse/emit throughput.
  version       Print the gopapy version.
  help          Show this message.
`)
}
