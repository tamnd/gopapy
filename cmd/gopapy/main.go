// Command gopapy parses Python source and emits an AST compatible with
// CPython's ast.dump output.
package main

import (
	"fmt"
	"io"
	"os"
)

const version = "0.0.1"

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
	default:
		return fmt.Errorf("unknown command %q (try 'gopapy help')", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: gopapy <command> [args]

Commands:
  parse FILE    Parse a Python source file; exit non-zero on error.
  ast   FILE    Print AST as JSON (matches python -c 'import ast; ...').
  dump  FILE    Print AST in ast.dump style.
  check DIR     Parse every .py under DIR, summarise failures.
  version       Print the gopapy version.
  help          Show this message.
`)
}
