package linter

import (
	"sort"

	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
	"github.com/tamnd/gopapy/symbols"
)

func checkF821(sm *symbols.Module, mod *parser.Module) []diag.Diagnostic {
	if sm == nil || sm.Root == nil || mod == nil {
		return nil
	}
	if hasModuleStarImport(mod) {
		return nil
	}
	var out []diag.Diagnostic
	var walk func(s *symbols.Scope)
	walk = func(s *symbols.Scope) {
		for name, sym := range s.Symbols {
			if !sym.Has(symbols.FlagUsed) {
				continue
			}
			if sym.Has(symbols.FlagBound) || sym.Has(symbols.FlagGlobal) || sym.Has(symbols.FlagNonlocal) {
				continue
			}
			if builtinNames[name] {
				continue
			}
			if found, _, _ := s.Resolve(name); found != nil {
				continue
			}
			for _, pos := range sym.UseSites {
				out = append(out, diag.Diagnostic{
					Pos:      pos,
					End:      pos,
					Severity: diag.SeverityWarning,
					Code:     CodeUndefinedName,
					Msg:      "undefined name '" + name + "'",
				})
			}
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(sm.Root)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Pos, out[j].Pos
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	return out
}

func hasModuleStarImport(mod *parser.Module) bool {
	for _, s := range mod.Body {
		imp, ok := s.(*parser.ImportFrom)
		if !ok {
			continue
		}
		for _, a := range imp.Names {
			if a.Name == "*" {
				return true
			}
		}
	}
	return false
}

var builtinNames = map[string]bool{
	"abs": true, "aiter": true, "all": true, "anext": true, "any": true,
	"ascii": true, "bin": true, "breakpoint": true, "callable": true,
	"chr": true, "compile": true, "delattr": true, "dir": true,
	"divmod": true, "enumerate": true, "eval": true, "exec": true,
	"exit": true, "filter": true, "format": true, "getattr": true,
	"globals": true, "hasattr": true, "hash": true, "help": true,
	"hex": true, "id": true, "input": true, "isinstance": true,
	"issubclass": true, "iter": true, "len": true, "locals": true,
	"map": true, "max": true, "min": true, "next": true, "oct": true,
	"open": true, "ord": true, "pow": true, "print": true, "quit": true,
	"repr": true, "reversed": true, "round": true, "setattr": true,
	"sorted": true, "sum": true, "vars": true, "zip": true,
	"__import__": true, "__build_class__": true,
	"bool": true, "bytearray": true, "bytes": true, "classmethod": true,
	"complex": true, "dict": true, "float": true, "frozenset": true,
	"int": true, "list": true, "memoryview": true, "object": true,
	"property": true, "range": true, "set": true, "slice": true,
	"staticmethod": true, "str": true, "super": true, "tuple": true,
	"type": true,
	"True": true, "False": true, "None": true, "NotImplemented": true,
	"Ellipsis": true, "__debug__": true, "__name__": true,
	"__file__": true, "__doc__": true, "__package__": true,
	"__loader__": true, "__spec__": true, "__builtins__": true,
	"copyright": true, "credits": true, "license": true,
	"BaseException": true, "Exception": true, "ArithmeticError": true,
	"AssertionError": true, "AttributeError": true,
	"BaseExceptionGroup": true, "BlockingIOError": true,
	"BrokenPipeError": true, "BufferError": true, "BytesWarning": true,
	"ChildProcessError": true, "ConnectionAbortedError": true,
	"ConnectionError": true, "ConnectionRefusedError": true,
	"ConnectionResetError": true, "DeprecationWarning": true,
	"EOFError": true, "EncodingWarning": true, "EnvironmentError": true,
	"ExceptionGroup": true, "FileExistsError": true,
	"FileNotFoundError": true, "FloatingPointError": true,
	"FutureWarning": true, "GeneratorExit": true, "IOError": true,
	"ImportError": true, "ImportWarning": true, "IndentationError": true,
	"IndexError": true, "InterruptedError": true, "IsADirectoryError": true,
	"KeyError": true, "KeyboardInterrupt": true, "LookupError": true,
	"MemoryError": true, "ModuleNotFoundError": true, "NameError": true,
	"NotADirectoryError": true, "NotImplementedError": true,
	"OSError": true, "OverflowError": true,
	"PendingDeprecationWarning": true, "PermissionError": true,
	"ProcessLookupError": true, "RecursionError": true,
	"ReferenceError": true, "ResourceWarning": true, "RuntimeError": true,
	"RuntimeWarning": true, "StopAsyncIteration": true,
	"StopIteration": true, "SyntaxError": true, "SyntaxWarning": true,
	"SystemError": true, "SystemExit": true, "TabError": true,
	"TimeoutError": true, "TypeError": true, "UnboundLocalError": true,
	"UnicodeDecodeError": true, "UnicodeEncodeError": true,
	"UnicodeError": true, "UnicodeTranslateError": true,
	"UnicodeWarning": true, "UserWarning": true, "ValueError": true,
	"Warning": true, "ZeroDivisionError": true,
	"PythonFinalizationError": true,
}
