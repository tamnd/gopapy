// Command gen-ast regenerates ast/*_gen.go from internal/asdlgen/Python.asdl.
//
//	go run ./internal/asdlgen/cmd/gen-ast
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tamnd/gopapy/internal/asdlgen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen-ast:", err)
		os.Exit(1)
	}
}

func run() error {
	asdlPath := "internal/asdlgen/Python.asdl"
	outDir := "ast"
	src, err := os.ReadFile(asdlPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", asdlPath, err)
	}
	mod, err := asdlgen.Parse(string(src))
	if err != nil {
		return fmt.Errorf("parse asdl: %w", err)
	}
	files, err := asdlgen.Generate(mod, "ast")
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for name, body := range files {
		dst := filepath.Join(outDir, name)
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		fmt.Println("wrote", dst)
	}
	return nil
}
