// Command gopapy parses Python source and emits an AST compatible with
// CPython's ast.dump output.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/gopapy/ast"
	"github.com/tamnd/gopapy/parser"
)

const version = "0.0.8"

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
	case "check":
		if len(args) < 2 {
			return fmt.Errorf("check: missing DIR argument")
		}
		return checkDir(args[1], stdout, stderr)
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
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".py") {
			return nil
		}
		if _, perr := parseFile(path); perr != nil {
			failed++
			fmt.Fprintf(stderr, "FAIL %s: %v\n", path, perr)
		} else {
			passed++
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

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: gopapy <command> [args]

Commands:
  parse FILE    Parse a Python source file; exit non-zero on error.
  dump  FILE    Print AST in ast.dump style.
  check DIR     Parse every .py under DIR, summarise failures.
  version       Print the gopapy version.
  help          Show this message.
`)
}
